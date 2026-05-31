package smtpd

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"
)

func prependHeaderToSpool(spooled *os.File, header string) (*os.File, int64, error) {
	if _, err := spooled.Seek(0, io.SeekStart); err != nil {
		return nil, 0, fmt.Errorf("rewind spooled message for header prepend: %w", err)
	}
	prefixed, err := os.CreateTemp("", "gogomail-spool-*.eml")
	if err != nil {
		return nil, 0, fmt.Errorf("create prefixed spool: %w", err)
	}
	written, err := io.WriteString(prefixed, header)
	if err != nil {
		cleanupSpool(prefixed)
		return nil, 0, fmt.Errorf("write received header: %w", err)
	}
	copied, err := io.Copy(prefixed, spooled)
	if err != nil {
		cleanupSpool(prefixed)
		return nil, 0, fmt.Errorf("copy spooled message after received header: %w", err)
	}
	return prefixed, int64(written) + copied, nil
}

func insertHeaderAfterTraceHeaders(spooled *os.File, header string) (*os.File, int64, error) {
	if _, err := spooled.Seek(0, io.SeekStart); err != nil {
		return nil, 0, fmt.Errorf("rewind spooled message for header insert: %w", err)
	}
	updated, err := os.CreateTemp("", "gogomail-spool-*.eml")
	if err != nil {
		return nil, 0, fmt.Errorf("create updated spool: %w", err)
	}

	var written int64
	writeString := func(value string) error {
		n, err := io.WriteString(updated, value)
		written += int64(n)
		return err
	}

	reader := bufio.NewReader(spooled)
	inserted := false
	inTrace := false
	for {
		line, err := readSMTPSpoolLine(reader)
		if line != "" {
			isContinuation := len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
			isReceived := strings.HasPrefix(strings.ToLower(line), "received:")
			if !inserted && !isReceived && !(inTrace && isContinuation) {
				if err := writeString(header); err != nil {
					cleanupSpool(updated)
					return nil, 0, fmt.Errorf("write inserted header: %w", err)
				}
				inserted = true
			}
			if err := writeString(line); err != nil {
				cleanupSpool(updated)
				return nil, 0, fmt.Errorf("copy spooled header line: %w", err)
			}
			inTrace = isReceived || (inTrace && isContinuation)
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			break
		}
		cleanupSpool(updated)
		return nil, 0, fmt.Errorf("read spooled message for header insert: %w", err)
	}
	if !inserted {
		if err := writeString(header); err != nil {
			cleanupSpool(updated)
			return nil, 0, fmt.Errorf("write inserted header: %w", err)
		}
	}
	return updated, written, nil
}

func cleanupSpool(spooled *os.File) {
	if spooled == nil {
		return
	}
	_ = spooled.Close()
	_ = os.Remove(spooled.Name())
}

func spoolMessage(r io.Reader, maxBytes int64) (*os.File, int64, error) {
	file, err := os.CreateTemp("", "gogomail-smtp-*.eml")
	if err != nil {
		return nil, 0, fmt.Errorf("create smtp spool file: %w", err)
	}

	limited := io.LimitReader(r, maxBytes+1)
	size, copyErr := io.Copy(file, limited)
	if copyErr != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, 0, fmt.Errorf("spool smtp message: %w", copyErr)
	}
	if size > maxBytes {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, size, gosmtp.ErrDataTooLarge
	}
	return file, size, nil
}

func randomMessageID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), hex.EncodeToString(random[:]))
}

func BuildStoragePath(mailbox Mailbox, messageID string, receivedAt time.Time) string {
	return strings.Join([]string{
		"mailstore",
		sanitizeStorageSegment(mailbox.CompanyID),
		sanitizeStorageSegment(mailbox.DomainID),
		sanitizeStorageSegment(mailbox.UserID),
		"maildir",
		receivedAt.Format("2006"),
		receivedAt.Format("01"),
		sanitizeStorageSegment(messageID) + ".eml",
	}, "/")
}

func sanitizeStorageSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "_"
	}
	var b strings.Builder
	b.Grow(len(value))
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "_"
	}
	return out
}
