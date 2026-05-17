package attachmentscan

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

const (
	defaultClamAVChunkBytes = 64 << 10
	maxClamAVResponseBytes  = 4 << 10
)

type ClamAVOptions struct {
	Addr    string
	Timeout time.Duration
	Dialer  func(ctx context.Context, network, address string) (net.Conn, error)
}

type ClamAVScanner struct {
	addr    string
	timeout time.Duration
	dialer  func(ctx context.Context, network, address string) (net.Conn, error)
}

func NewClamAVScanner(opts ClamAVOptions) (*ClamAVScanner, error) {
	addr := strings.TrimSpace(opts.Addr)
	if addr == "" || strings.ContainsAny(addr, "\r\n\t ") {
		return nil, fmt.Errorf("clamav address must be a non-empty host:port without whitespace")
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return nil, fmt.Errorf("clamav address must be host:port: %w", err)
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := opts.Dialer
	if dialer == nil {
		nd := &net.Dialer{Timeout: timeout}
		dialer = nd.DialContext
	}
	return &ClamAVScanner{addr: addr, timeout: timeout, dialer: dialer}, nil
}

func (s *ClamAVScanner) ScanStream(ctx context.Context, name string, file *os.File) (Result, error) {
	if s == nil || s.dialer == nil {
		return Result{}, fmt.Errorf("clamav scanner is not configured")
	}
	if file == nil {
		return Result{}, fmt.Errorf("clamav scan requires a file")
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	conn, err := s.dialer(ctx, "tcp", s.addr)
	if err != nil {
		return Result{}, fmt.Errorf("connect clamd: %w", err)
	}
	defer conn.Close()
	deadline, ok := ctx.Deadline()
	if ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err := io.WriteString(conn, "zINSTREAM\x00"); err != nil {
		return Result{}, fmt.Errorf("start clamd instream: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return Result{}, fmt.Errorf("rewind scan stream: %w", err)
	}
	buf := make([]byte, defaultClamAVChunkBytes)
	var prefix [4]byte
	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			binary.BigEndian.PutUint32(prefix[:], uint32(n))
			if _, err := conn.Write(prefix[:]); err != nil {
				return Result{}, fmt.Errorf("write clamd chunk length: %w", err)
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				return Result{}, fmt.Errorf("write clamd chunk: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return Result{}, fmt.Errorf("read scan stream: %w", readErr)
		}
	}
	binary.BigEndian.PutUint32(prefix[:], 0)
	if _, err := conn.Write(prefix[:]); err != nil {
		return Result{}, fmt.Errorf("finish clamd instream: %w", err)
	}
	response, err := bufio.NewReader(io.LimitReader(conn, maxClamAVResponseBytes+1)).ReadString('\n')
	if err != nil && err != io.EOF {
		return Result{}, fmt.Errorf("read clamd response: %w", err)
	}
	if len(response) > maxClamAVResponseBytes {
		return Result{}, fmt.Errorf("clamd response is too large")
	}
	return parseClamAVResponse(response), nil
}

func parseClamAVResponse(response string) Result {
	response = strings.TrimSpace(response)
	switch {
	case response == "" || strings.HasSuffix(response, ": OK"):
		return Result{Verdict: VerdictAccept}
	case strings.HasSuffix(response, " FOUND"):
		name := strings.TrimSuffix(response, " FOUND")
		if idx := strings.LastIndex(name, ": "); idx >= 0 {
			name = name[idx+2:]
		}
		return Result{Verdict: VerdictReject, Reason: "clamav detected " + cleanReason(name)}
	case strings.Contains(response, "ERROR"):
		return Result{Verdict: VerdictTempfail, Reason: "clamav error"}
	default:
		return Result{Verdict: VerdictTempfail, Reason: "unexpected clamav response"}
	}
}
