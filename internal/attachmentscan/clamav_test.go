package attachmentscan

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

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
	err := hook(context.Background(), smtpd.Event{Stage: smtpd.StageSpooled, SpoolPath: file.Name()})
	if err == nil {
		t.Fatal("hook accepted rejected stream")
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
