package attachmentscan

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/message"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestClamAVScannerAcceptsCleanStream(t *testing.T) {
	t.Parallel()

	server := newFakeClamd(t, "stream: OK\n")
	scanner, err := NewClamAVScanner(ClamAVOptions{Addr: server.addr, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClamAVScanner returned error: %v", err)
	}
	file := tempScanFile(t, "clean attachment")
	result, err := scanner.ScanStream(context.Background(), "message.eml", file)
	if err != nil {
		t.Fatalf("ScanStream returned error: %v", err)
	}
	if result.Verdict != VerdictAccept {
		t.Fatalf("verdict = %q, want accept", result.Verdict)
	}
	if got := <-server.body; got != "clean attachment" {
		t.Fatalf("scanned body = %q", got)
	}
}

func TestClamAVScannerRejectsFoundStream(t *testing.T) {
	t.Parallel()

	server := newFakeClamd(t, "stream: Eicar-Test-Signature FOUND\n")
	scanner, err := NewClamAVScanner(ClamAVOptions{Addr: server.addr, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClamAVScanner returned error: %v", err)
	}
	result, err := scanner.ScanStream(context.Background(), "message.eml", tempScanFile(t, "virus"))
	if err != nil {
		t.Fatalf("ScanStream returned error: %v", err)
	}
	if result.Verdict != VerdictReject || !strings.Contains(result.Reason, "Eicar-Test-Signature") {
		t.Fatalf("result = %+v, want reject with signature", result)
	}
}

func TestStreamHookScansSpooledFile(t *testing.T) {
	t.Parallel()

	file := tempScanFile(t, "body")
	hook := StreamHook(StreamHookOptions{Scanner: fakeStreamScanner{result: Result{Verdict: VerdictReject, Reason: "bad"}}})
	err := hook(context.Background(), smtpd.Event{
		Stage:     smtpd.StageParsed,
		SpoolPath: file.Name(),
		Parsed:    messageWithAttachment(),
	})
	if err == nil {
		t.Fatal("hook accepted rejected stream")
	}
}

func TestStreamHookSkipsMessagesWithoutAttachments(t *testing.T) {
	t.Parallel()

	file := tempScanFile(t, "body")
	scanner := &countingStreamScanner{}
	hook := StreamHook(StreamHookOptions{Scanner: scanner})
	if err := hook(context.Background(), smtpd.Event{Stage: smtpd.StageParsed, SpoolPath: file.Name()}); err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	if scanner.calls != 0 {
		t.Fatalf("stream scans = %d, want 0", scanner.calls)
	}
}

func TestClamAVScannerReturnsTempfailWhenSaturated(t *testing.T) {
	t.Parallel()

	dialStarted := make(chan struct{})
	releaseDial := make(chan struct{})
	scanner, err := NewClamAVScanner(ClamAVOptions{
		Addr:           "127.0.0.1:3310",
		Timeout:        time.Second,
		MaxConcurrency: 1,
		Dialer: func(ctx context.Context, network, address string) (net.Conn, error) {
			close(dialStarted)
			select {
			case <-releaseDial:
				return nil, fmt.Errorf("released")
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	})
	if err != nil {
		t.Fatalf("NewClamAVScanner returned error: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_, _ = scanner.ScanStream(context.Background(), "first.eml", tempScanFile(t, "first"))
		close(done)
	}()
	<-dialStarted
	result, err := scanner.ScanStream(context.Background(), "second.eml", tempScanFile(t, "second"))
	if err != nil {
		t.Fatalf("ScanStream returned error: %v", err)
	}
	if result.Verdict != VerdictTempfail || !strings.Contains(result.Reason, "saturated") {
		t.Fatalf("result = %+v, want saturated tempfail", result)
	}
	close(releaseDial)
	<-done
}

func TestClamAVScannerCircuitBreakerOpensAfterFailures(t *testing.T) {
	t.Parallel()

	calls := 0
	scanner, err := NewClamAVScanner(ClamAVOptions{
		Addr:             "127.0.0.1:3310",
		Timeout:          time.Second,
		FailureThreshold: 1,
		Dialer: func(context.Context, string, string) (net.Conn, error) {
			calls++
			return nil, fmt.Errorf("down")
		},
	})
	if err != nil {
		t.Fatalf("NewClamAVScanner returned error: %v", err)
	}
	result, err := scanner.ScanStream(context.Background(), "first.eml", tempScanFile(t, "first"))
	if err != nil || result.Verdict != VerdictTempfail {
		t.Fatalf("first result = %+v err=%v, want tempfail", result, err)
	}
	result, err = scanner.ScanStream(context.Background(), "second.eml", tempScanFile(t, "second"))
	if err != nil {
		t.Fatalf("second ScanStream returned error: %v", err)
	}
	if result.Verdict != VerdictTempfail || !strings.Contains(result.Reason, "circuit open") {
		t.Fatalf("second result = %+v, want circuit open tempfail", result)
	}
	if calls != 1 {
		t.Fatalf("dial calls = %d, want 1", calls)
	}
}

func TestClamAVScannerBoundsScanBytes(t *testing.T) {
	t.Parallel()

	scanner, err := NewClamAVScanner(ClamAVOptions{
		Addr:         "127.0.0.1:3310",
		Timeout:      time.Second,
		MaxScanBytes: 2,
		Dialer: func(context.Context, string, string) (net.Conn, error) {
			server, client := net.Pipe()
			go func() {
				defer server.Close()
				_, _ = io.Copy(io.Discard, server)
			}()
			return client, nil
		},
	})
	if err != nil {
		t.Fatalf("NewClamAVScanner returned error: %v", err)
	}
	result, err := scanner.ScanStream(context.Background(), "large.eml", tempScanFile(t, "large"))
	if err != nil {
		t.Fatalf("ScanStream returned error: %v", err)
	}
	if result.Verdict != VerdictTempfail || !strings.Contains(result.Reason, "too large") {
		t.Fatalf("result = %+v, want too-large tempfail", result)
	}
}

func tempScanFile(t *testing.T, body string) *os.File {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "scan-*.eml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(body); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	return file
}

func messageWithAttachment() message.ParsedMessage {
	return message.ParsedMessage{HasAttachment: true, Attachments: []message.Attachment{{Filename: "report.pdf"}}}
}

type fakeClamd struct {
	addr string
	body chan string
}

func newFakeClamd(t *testing.T, response string) fakeClamd {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := fakeClamd{addr: ln.Addr().String(), body: make(chan string, 1)}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		command := make([]byte, len("zINSTREAM\x00"))
		if _, err := io.ReadFull(conn, command); err != nil {
			return
		}
		var payload strings.Builder
		for {
			var lengthBytes [4]byte
			if _, err := io.ReadFull(conn, lengthBytes[:]); err != nil {
				return
			}
			length := binary.BigEndian.Uint32(lengthBytes[:])
			if length == 0 {
				break
			}
			chunk := make([]byte, length)
			if _, err := io.ReadFull(conn, chunk); err != nil {
				return
			}
			payload.Write(chunk)
		}
		server.body <- payload.String()
		_, _ = io.WriteString(conn, response)
	}()
	return server
}

type fakeStreamScanner struct {
	result Result
}

func (s fakeStreamScanner) ScanStream(context.Context, string, *os.File) (Result, error) {
	return s.result, nil
}

type countingStreamScanner struct {
	calls int
}

func (s *countingStreamScanner) ScanStream(context.Context, string, *os.File) (Result, error) {
	s.calls++
	return Result{Verdict: VerdictAccept}, nil
}
