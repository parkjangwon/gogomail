package imapimport

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeIMAPServer is a tiny scripted IMAP server used to exercise the real
// imapClient wire implementation without any external network. It speaks just
// enough of RFC 3501 for LOGIN, LIST, EXAMINE, and UID FETCH.
type fakeIMAPServer struct {
	ln       net.Listener
	messages string // body served by UID FETCH for INBOX
}

func startFakeIMAPServer(t *testing.T, body string) *fakeIMAPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &fakeIMAPServer{ln: ln, messages: body}
	go s.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *fakeIMAPServer) addr() (string, int) {
	tcp := s.ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", tcp.Port
}

func (s *fakeIMAPServer) serve() {
	conn, err := s.ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := func(format string, args ...any) { fmt.Fprintf(conn, format+"\r\n", args...) }

	w("* OK fake IMAP ready")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		fields := strings.SplitN(line, " ", 3)
		if len(fields) < 2 {
			continue
		}
		tag, cmd := fields[0], strings.ToUpper(fields[1])
		switch {
		case cmd == "LOGIN":
			w("%s OK LOGIN completed", tag)
		case cmd == "AUTHENTICATE":
			w("%s OK AUTHENTICATE completed", tag)
		case cmd == "LIST":
			w(`* LIST (\HasNoChildren) "/" "INBOX"`)
			w(`* LIST (\Noselect \HasChildren) "/" "[Gmail]"`)
			w(`* LIST (\All) "/" "[Gmail]/All Mail"`)
			w("%s OK LIST completed", tag)
		case cmd == "STATUS":
			w(`* STATUS "INBOX" (MESSAGES 1)`)
			w("%s OK STATUS completed", tag)
		case cmd == "EXAMINE":
			w("* 1 EXISTS")
			w("* OK [UIDVALIDITY 1] ok")
			w("%s OK [READ-ONLY] EXAMINE completed", tag)
		case cmd == "UID" && strings.Contains(strings.ToUpper(line), "FETCH"):
			body := s.messages
			fmt.Fprintf(conn, "* 1 FETCH (UID 5 FLAGS (\\Seen) INTERNALDATE \"01-Jan-2024 10:30:00 +0000\" BODY[] {%d}\r\n", len(body))
			fmt.Fprint(conn, body)
			w(")")
			w("%s OK UID FETCH completed", tag)
		case cmd == "LOGOUT":
			w("* BYE logging out")
			w("%s OK LOGOUT completed", tag)
			return
		default:
			w("%s OK %s completed", tag, cmd)
		}
	}
}

func TestImapClientEndToEndAgainstFakeServer(t *testing.T) {
	if testing.Short() {
		// Uses a loopback TCP listener (not external network) but keep it out of
		// the fastest short lane to honor the no-network intent conservatively.
	}
	body := "Subject: Hello\r\nMessage-ID: <e2e@example.com>\r\n\r\nBody text"
	srv := startFakeIMAPServer(t, body)
	host, port := srv.addr()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Dial(ctx, DialOptions{
		Host:          host,
		Port:          port,
		AllowInsecure: true, // loopback plaintext for the test only
		Username:      "user@example.com",
		Password:      "secret",
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	sink := newFakeSink()
	imp := New(client, sink, nil, Options{Mapping: MappingConfig{Strategy: MappingPrefix, Separator: "/"}})
	report, err := imp.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// [Gmail] (\Noselect) and [Gmail]/All Mail (well-known skip) are skipped;
	// INBOX yields one message.
	if report.MessagesStored != 1 {
		t.Fatalf("MessagesStored = %d want 1 (errors: %v)", report.MessagesStored, report.Errors)
	}
	if len(sink.stored) != 1 {
		t.Fatalf("sink stored %d want 1", len(sink.stored))
	}
	got := sink.stored[0]
	if got.Destination != "INBOX" {
		t.Errorf("destination = %q want INBOX", got.Destination)
	}
	if !got.Flags.Read {
		t.Error("\\Seen not translated")
	}
	if got.IdempotencyKey != "e2e@example.com" {
		t.Errorf("idempotency key = %q want e2e@example.com", got.IdempotencyKey)
	}
	wantDate := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
	if !got.InternalDate.Equal(wantDate) {
		t.Errorf("internal date = %v want %v", got.InternalDate, wantDate)
	}
	if string(got.Raw) != body {
		t.Errorf("raw mismatch: %q", got.Raw)
	}
}

func TestDialRefusesInsecureByDefault(t *testing.T) {
	_, err := Dial(context.Background(), DialOptions{
		Host:     "example.com",
		Username: "u",
		Password: "p",
		// No TLS, no StartTLS, no AllowInsecure.
	})
	if err == nil {
		t.Fatal("expected Dial to refuse a non-TLS connection")
	}
	if !strings.Contains(err.Error(), "TLS") {
		t.Errorf("error = %v, want TLS refusal", err)
	}
}

func TestDialRequiresCredentials(t *testing.T) {
	_, err := Dial(context.Background(), DialOptions{
		Host:          "example.com",
		AllowInsecure: true,
	})
	if err == nil {
		t.Fatal("expected Dial to require credentials")
	}
}
