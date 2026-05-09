package milterhook_test

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/message"
	"github.com/gogomail/gogomail/internal/milter"
	"github.com/gogomail/gogomail/internal/milterhook"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

// sendRaw writes a single milter packet to w.
func sendRaw(w io.Writer, cmd byte, data []byte) {
	size := uint32(1 + len(data))
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], size)
	w.Write(hdr[:])
	w.Write([]byte{cmd})
	if len(data) > 0 {
		w.Write(data)
	}
}

// stubServer acts as a milter server: auto-handles OPTNEG, sends finalAction for all other commands.
func stubServer(conn net.Conn, finalAction byte) {
	defer conn.Close()
	for {
		var hdr [4]byte
		if _, err := io.ReadFull(conn, hdr[:]); err != nil {
			return
		}
		size := binary.BigEndian.Uint32(hdr[:])
		if size == 0 {
			return
		}
		buf := make([]byte, size)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		switch buf[0] {
		case 'O': // OPTNEG
			resp := make([]byte, 12)
			binary.BigEndian.PutUint32(resp[0:4], 6)
			binary.BigEndian.PutUint32(resp[4:8], 0x7F)
			binary.BigEndian.PutUint32(resp[8:12], 0)
			sendRaw(conn, 'O', resp)
		case 'Q', 'K': // QUIT
			return
		case 'A': // ABORT — no response per spec
		default:
			sendRaw(conn, finalAction, nil)
		}
	}
}

// pipeDialer returns a Dialer that connects to a stub server via net.Pipe.
func pipeDialer(t *testing.T, serverFn func(conn net.Conn)) milterhook.Dialer {
	t.Helper()
	return func(ctx context.Context) (*milter.Client, error) {
		clientConn, serverConn := net.Pipe()
		go serverFn(serverConn)
		return milter.NewClient(clientConn, 5*time.Second), nil
	}
}

func parsedEvent() smtpd.Event {
	return smtpd.Event{
		Stage:        smtpd.StageParsed,
		RemoteAddr:   "1.2.3.4:12345",
		EnvelopeFrom: "<sender@example.com>",
		Recipients:   []string{"<rcpt@example.com>"},
		Parsed: message.ParsedMessage{
			MessageID: "abc123@example.com",
			Subject:   "Hello",
			From:      message.Address{Address: "sender@example.com"},
			TextBody:  "Hello world",
		},
	}
}

func TestHookSkipsNonParsedStage(t *testing.T) {
	called := false
	dialer := milterhook.Dialer(func(ctx context.Context) (*milter.Client, error) {
		called = true
		return nil, nil
	})
	h := milterhook.Hook(milterhook.HookOptions{Dialer: dialer})

	for _, stage := range []smtpd.Stage{
		smtpd.StageAuthenticated,
		smtpd.StageAuthenticationChecked,
		smtpd.StageStored,
	} {
		ev := parsedEvent()
		ev.Stage = stage
		if err := h(context.Background(), ev); err != nil {
			t.Fatalf("stage %s: unexpected error: %v", stage, err)
		}
		if called {
			t.Fatalf("stage %s: dialer was called unexpectedly", stage)
		}
	}
}

func TestHookNilDialer(t *testing.T) {
	h := milterhook.Hook(milterhook.HookOptions{Dialer: nil})
	if err := h(context.Background(), parsedEvent()); err != nil {
		t.Fatalf("nil dialer: unexpected error: %v", err)
	}
}

func TestHookAccept(t *testing.T) {
	h := milterhook.Hook(milterhook.HookOptions{
		Dialer: pipeDialer(t, func(conn net.Conn) { stubServer(conn, 'a') }),
	})
	if err := h(context.Background(), parsedEvent()); err != nil {
		t.Fatalf("accept: unexpected error: %v", err)
	}
}

func TestHookContinue(t *testing.T) {
	h := milterhook.Hook(milterhook.HookOptions{
		Dialer: pipeDialer(t, func(conn net.Conn) { stubServer(conn, 'c') }),
	})
	if err := h(context.Background(), parsedEvent()); err != nil {
		t.Fatalf("continue: unexpected error: %v", err)
	}
}

func TestHookReject(t *testing.T) {
	h := milterhook.Hook(milterhook.HookOptions{
		Dialer: pipeDialer(t, func(conn net.Conn) { stubServer(conn, 'r') }),
	})
	if err := h(context.Background(), parsedEvent()); err == nil {
		t.Fatal("reject: expected error, got nil")
	}
}

func TestHookTempfail(t *testing.T) {
	h := milterhook.Hook(milterhook.HookOptions{
		Dialer: pipeDialer(t, func(conn net.Conn) { stubServer(conn, 't') }),
	})
	if err := h(context.Background(), parsedEvent()); err == nil {
		t.Fatal("tempfail: expected error, got nil")
	}
}

func TestHookDiscard(t *testing.T) {
	h := milterhook.Hook(milterhook.HookOptions{
		Dialer: pipeDialer(t, func(conn net.Conn) { stubServer(conn, 'd') }),
	})
	if err := h(context.Background(), parsedEvent()); err != nil {
		t.Fatalf("discard: unexpected error: %v", err)
	}
}

func TestHookIPv6Address(t *testing.T) {
	h := milterhook.Hook(milterhook.HookOptions{
		Dialer: pipeDialer(t, func(conn net.Conn) { stubServer(conn, 'a') }),
	})
	ev := parsedEvent()
	ev.RemoteAddr = "[::1]:25"
	if err := h(context.Background(), ev); err != nil {
		t.Fatalf("IPv6: unexpected error: %v", err)
	}
}

func TestHookMultipleRecipients(t *testing.T) {
	h := milterhook.Hook(milterhook.HookOptions{
		Dialer: pipeDialer(t, func(conn net.Conn) { stubServer(conn, 'a') }),
	})
	ev := parsedEvent()
	ev.Recipients = []string{"<a@example.com>", "<b@example.com>", "<c@example.com>"}
	if err := h(context.Background(), ev); err != nil {
		t.Fatalf("multi-rcpt: unexpected error: %v", err)
	}
}

func TestHookNoBody(t *testing.T) {
	h := milterhook.Hook(milterhook.HookOptions{
		Dialer: pipeDialer(t, func(conn net.Conn) { stubServer(conn, 'a') }),
	})
	ev := parsedEvent()
	ev.Parsed.TextBody = ""
	if err := h(context.Background(), ev); err != nil {
		t.Fatalf("no body: unexpected error: %v", err)
	}
}

func TestNetworkDialer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		stubServer(conn, 'a')
	}()

	d := milterhook.NetworkDialer(ln.Addr().String(), 5*time.Second)
	h := milterhook.Hook(milterhook.HookOptions{Dialer: d})
	if err := h(context.Background(), parsedEvent()); err != nil {
		t.Fatalf("network dialer: %v", err)
	}
	<-done
}
