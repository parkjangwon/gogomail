package imapimport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// rateLimiter is a simple token-bucket-ish limiter that spaces operations to at
// most `rate` per second. A rate <= 0 means unlimited.
type rateLimiter struct {
	interval time.Duration
	next     time.Time
	ch       chan struct{}
}

func newRateLimiter(rate float64) *rateLimiter {
	if rate <= 0 {
		return &rateLimiter{}
	}
	rl := &rateLimiter{
		interval: time.Duration(float64(time.Second) / rate),
		ch:       make(chan struct{}, 1),
	}
	rl.ch <- struct{}{}
	return rl
}

// wait blocks until the next operation is permitted or ctx is canceled.
func (rl *rateLimiter) wait(ctx context.Context) error {
	if rl == nil || rl.interval == 0 {
		return nil
	}
	// Serialize access to next via the 1-slot channel.
	select {
	case <-rl.ch:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { rl.ch <- struct{}{} }()

	now := time.Now()
	if rl.next.IsZero() || now.After(rl.next) {
		rl.next = now.Add(rl.interval)
		return nil
	}
	delay := time.Until(rl.next)
	rl.next = rl.next.Add(rl.interval)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// messageIDFromRaw extracts the RFC 5322 Message-ID header value from raw
// message bytes without a full MIME parse. It scans only the header block
// (up to the first blank line) so it stays cheap on the import hot path.
func messageIDFromRaw(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	// Bound the header scan to avoid pathological inputs.
	const maxHeaderScan = 1 << 20 // 1 MiB
	head := raw
	if len(head) > maxHeaderScan {
		head = head[:maxHeaderScan]
	}
	sc := bufio.NewScanner(bytes.NewReader(head))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var current string
	collecting := false
	var value strings.Builder
	flush := func() string {
		if collecting {
			return strings.TrimSpace(value.String())
		}
		return ""
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			break // end of headers
		}
		// Folded continuation line.
		if collecting && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			value.WriteString(" ")
			value.WriteString(strings.TrimSpace(line))
			continue
		}
		if collecting {
			return flush()
		}
		if idx := strings.IndexByte(line, ':'); idx > 0 {
			current = strings.ToLower(strings.TrimSpace(line[:idx]))
			if current == "message-id" {
				collecting = true
				value.WriteString(strings.TrimSpace(line[idx+1:]))
			}
		}
	}
	return flush()
}

// FileProgressStore persists ProgressSnapshot as JSON to a file. Save writes to
// a temp file and renames for atomicity so an interrupted write cannot corrupt
// the resume state.
type FileProgressStore struct {
	Path string
}

// NewFileProgressStore returns a FileProgressStore at path.
func NewFileProgressStore(path string) *FileProgressStore {
	return &FileProgressStore{Path: path}
}

// Load implements ProgressStore.
func (s *FileProgressStore) Load() (ProgressSnapshot, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return ProgressSnapshot{}, nil
		}
		return ProgressSnapshot{}, fmt.Errorf("read progress file: %w", err)
	}
	var snap ProgressSnapshot
	if len(bytes.TrimSpace(data)) == 0 {
		return ProgressSnapshot{}, nil
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return ProgressSnapshot{}, fmt.Errorf("parse progress file: %w", err)
	}
	return snap, nil
}

// Save implements ProgressStore.
func (s *FileProgressStore) Save(snap ProgressSnapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal progress: %w", err)
	}
	dir := filepath.Dir(s.Path)
	tmp, err := os.CreateTemp(dir, ".imap-import-progress-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp progress file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp progress file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp progress file: %w", err)
	}
	if err := os.Rename(tmpName, s.Path); err != nil {
		return fmt.Errorf("rename progress file: %w", err)
	}
	return nil
}
