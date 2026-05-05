package imapgw

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestNewServerValidatesListenerOptions(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		opts ServerOptions
	}{
		{name: "blank address", opts: ServerOptions{Backend: fakeBackend{}, AllowInsecureAuth: true}},
		{name: "linebreak address", opts: ServerOptions{Addr: ":1143\nbad", Backend: fakeBackend{}, AllowInsecureAuth: true}},
		{name: "missing port", opts: ServerOptions{Addr: "localhost", Backend: fakeBackend{}, AllowInsecureAuth: true}},
		{name: "missing backend", opts: ServerOptions{Addr: ":1143", AllowInsecureAuth: true}},
		{name: "tls required", opts: ServerOptions{Addr: ":1143", Backend: fakeBackend{}}},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := NewServer(tt.opts); err == nil {
				t.Fatal("NewServer error = nil, want rejection")
			}
		})
	}
}

func TestNewServerAcceptsTLSOrExplicitInsecureAuthPolicy(t *testing.T) {
	t.Parallel()

	insecure, err := NewServer(ServerOptions{Addr: " :1143 ", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer insecure returned error: %v", err)
	}
	if insecure.Options().Addr != ":1143" {
		t.Fatalf("Addr = %q, want trimmed address", insecure.Options().Addr)
	}

	secure, err := NewServer(ServerOptions{Addr: "localhost:1993", Backend: fakeBackend{}, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}})
	if err != nil {
		t.Fatalf("NewServer secure returned error: %v", err)
	}
	if secure.Options().TLSConfig == nil || secure.Options().AllowInsecureAuth {
		t.Fatalf("secure options = %+v", secure.Options())
	}
}

func TestServerListenUsesConfiguredAddress(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: "127.0.0.1:0", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	listener, err := server.Listen()
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	defer listener.Close()
	if listener.Addr().String() == "" {
		t.Fatal("listener address is empty")
	}
}

func TestServerHandlesGreetingCapabilityNoopAndLogout(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if line != "* OK gogomail IMAP4rev1 service ready\r\n" {
		t.Fatalf("greeting = %q", line)
	}

	if _, err := client.Write([]byte("a1 CAPABILITY\r\n")); err != nil {
		t.Fatalf("write capability: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read capability untagged: %v", err)
	}
	if line != "* CAPABILITY IMAP4rev1 IDLE AUTH=PLAIN\r\n" {
		t.Fatalf("capability = %q", line)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read capability tagged: %v", err)
	}
	if line != "a1 OK CAPABILITY completed\r\n" {
		t.Fatalf("capability completion = %q", line)
	}

	if _, err := client.Write([]byte("a2 NOOP\r\n")); err != nil {
		t.Fatalf("write noop: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read noop: %v", err)
	}
	if line != "a2 OK NOOP completed\r\n" {
		t.Fatalf("noop = %q", line)
	}

	if _, err := client.Write([]byte("a3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read bye: %v", err)
	}
	if line != "* BYE gogomail IMAP4rev1 server logging out\r\n" {
		t.Fatalf("bye = %q", line)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read logout completion: %v", err)
	}
	if line != "a3 OK LOGOUT completed\r\n" {
		t.Fatalf("logout completion = %q", line)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerServeStopsWhenListenerCloses(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: "127.0.0.1:0", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(listener)
	}()
	if err := listener.Close(); err != nil {
		t.Fatalf("listener close returned error: %v", err)
	}
	if err := <-errCh; err != ErrServerClosed {
		t.Fatalf("Serve returned %v, want ErrServerClosed", err)
	}
}

func TestServerHandlesLoginThroughBackend(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read login: %v", err)
	}
	if line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login = %q", line)
	}
	if _, err := client.Write([]byte("a2 CAPABILITY\r\n")); err != nil {
		t.Fatalf("write authenticated capability: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read authenticated capability untagged: %v", err)
	}
	if line != "* CAPABILITY IMAP4rev1 IDLE\r\n" {
		t.Fatalf("authenticated capability = %q", line)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read authenticated capability tagged: %v", err)
	}
	if line != "a2 OK CAPABILITY completed\r\n" {
		t.Fatalf("authenticated capability completion = %q", line)
	}
	if _, err := client.Write([]byte("a3 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write second login: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read second login: %v", err)
	}
	if line != "a3 BAD already authenticated\r\n" {
		t.Fatalf("second login = %q", line)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read bye: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read logout completion: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesQuotedLoginCredentials(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN \"user@example.com\" \"sec\\\\ret\"\r\na2 LOGOUT\r\n")); err != nil {
		t.Fatalf("write login/logout: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesAuthenticatePlain(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 AUTHENTICATE PLAIN\r\n")); err != nil {
		t.Fatalf("write authenticate: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read continuation: %v", err)
	}
	if line != "+ \r\n" {
		t.Fatalf("continuation = %q", line)
	}
	response := base64.StdEncoding.EncodeToString([]byte("\x00user@example.com\x00secret"))
	if _, err := client.Write([]byte(response + "\r\n")); err != nil {
		t.Fatalf("write authenticate response: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read authenticate completion: %v", err)
	}
	if line != "a1 OK AUTHENTICATE completed\r\n" {
		t.Fatalf("authenticate completion = %q", line)
	}
	if _, err := client.Write([]byte("a2 CAPABILITY\r\n")); err != nil {
		t.Fatalf("write capability: %v", err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 IDLE\r\n" {
		t.Fatalf("authenticated capability = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a2 OK CAPABILITY completed\r\n" {
		t.Fatalf("capability completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesSelectAfterLogin(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write unauthenticated select: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read unauthenticated select: %v", err)
	}
	if line != "a1 NO authentication required\r\n" {
		t.Fatalf("unauthenticated select = %q", line)
	}
	if _, err := client.Write([]byte("a2 LOGIN user@example.com secret\r\na3 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a2 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft)\r\n",
		"* 2 EXISTS\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft)] Permanent flags\r\n",
		"a3 OK [READ-WRITE] SELECT completed\r\n",
	}
	for _, expected := range want {
		line, err = reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read select response: %v", err)
		}
		if line != expected {
			t.Fatalf("select response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesExamineAsReadOnlySelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 EXAMINE inbox\r\n")); err != nil {
		t.Fatalf("write login/examine: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft)\r\n",
		"* 2 EXISTS\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [PERMANENTFLAGS ()] No permanent flags permitted\r\n",
		"a2 OK [READ-ONLY] EXAMINE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read examine response: %v", err)
		}
		if line != expected {
			t.Fatalf("examine response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a3 UID STORE 7 +FLAGS (\\Seen)\r\n")); err != nil {
		t.Fatalf("write uid store: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 NO mailbox is read-only\r\n" {
		t.Fatalf("read-only store line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesCheckAndCloseAfterSelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 CHECK\r\n")); err != nil {
		t.Fatalf("write check: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 OK CHECK completed\r\n" {
		t.Fatalf("check line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 CLOSE\r\n")); err != nil {
		t.Fatalf("write close: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a4 OK CLOSE completed\r\n" {
		t.Fatalf("close line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a5 FETCH 1 (FLAGS)\r\n")); err != nil {
		t.Fatalf("write fetch after close: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a5 NO mailbox must be selected\r\n" {
		t.Fatalf("fetch after close line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a6 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerNoopDrainsMailboxEvents(t *testing.T) {
	t.Parallel()

	backendImpl := &eventBackend{events: make(chan MailboxEvent, 4)}
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: backendImpl, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	backendImpl.events <- MailboxEvent{Type: MailboxEventExists, UserID: "user-1", MailboxID: "inbox", Messages: 3}
	backendImpl.events <- MailboxEvent{Type: MailboxEventFlags, UserID: "user-1", MailboxID: "inbox", UID: 7}
	if _, err := client.Write([]byte("a3 NOOP\r\n")); err != nil {
		t.Fatalf("write noop: %v", err)
	}
	want := []string{
		"* 3 EXISTS\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged))\r\n",
		"a3 OK NOOP completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read noop event response: %v", err)
		}
		if line != expected {
			t.Fatalf("noop event response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
	if !backendImpl.canceled {
		t.Fatal("event subscription was not canceled")
	}
}

func TestServerHandlesIdleDoneWithMailboxEvents(t *testing.T) {
	t.Parallel()

	backendImpl := &eventBackend{events: make(chan MailboxEvent, 4)}
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: backendImpl, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 IDLE\r\n")); err != nil {
		t.Fatalf("write idle: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ idling\r\n" {
		t.Fatalf("idle continuation = %q err = %v", line, err)
	}
	backendImpl.events <- MailboxEvent{Type: MailboxEventExists, UserID: "user-1", MailboxID: "inbox", Messages: 4}
	if _, err := client.Write([]byte("DONE\r\n")); err != nil {
		t.Fatalf("write done: %v", err)
	}
	want := []string{
		"* 4 EXISTS\r\n",
		"a3 OK IDLE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read idle response: %v", err)
		}
		if line != expected {
			t.Fatalf("idle response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesListAfterLogin(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" *\r\n")); err != nil {
		t.Fatalf("write login/list: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"Archive 2026\"\r\n",
		"a2 OK LIST completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read list response: %v", err)
		}
		if line != expected {
			t.Fatalf("list response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerFiltersListByPattern(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" \"INBOX\"\r\na3 LIST \"\" \"Archive%\"\r\n")); err != nil {
		t.Fatalf("write login/list: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"a2 OK LIST completed\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"Archive 2026\"\r\n",
		"a3 OK LIST completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read list response: %v", err)
		}
		if line != expected {
			t.Fatalf("list response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesStatusAfterLogin(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 STATUS inbox (MESSAGES UIDNEXT UIDVALIDITY UNSEEN)\r\n")); err != nil {
		t.Fatalf("write login/status: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* STATUS \"INBOX\" (MESSAGES 2 UIDNEXT 5 UIDVALIDITY 1 UNSEEN 1)\r\n",
		"a2 OK STATUS completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read status response: %v", err)
		}
		if line != expected {
			t.Fatalf("status response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesRequestedStatusItemsOnly(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 STATUS inbox (UIDNEXT RECENT)\r\na3 STATUS inbox (BADITEM)\r\n")); err != nil {
		t.Fatalf("write login/status: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* STATUS \"INBOX\" (UIDNEXT 5 RECENT 0)\r\n" {
		t.Fatalf("status subset line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 OK STATUS completed\r\n" {
		t.Fatalf("status subset completion = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 BAD STATUS item is unsupported\r\n" {
		t.Fatalf("bad status line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesUIDFetchAfterSelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 UID FETCH 7 (FLAGS RFC822.SIZE)\r\n")); err != nil {
		t.Fatalf("write login/uid fetch before select: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 NO mailbox must be selected\r\n" {
		t.Fatalf("preselect fetch line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a3 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a4 UID FETCH 7 (FLAGS RFC822.SIZE)\r\n")); err != nil {
		t.Fatalf("write uid fetch: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"a4 OK UID FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read uid fetch response: %v", err)
		}
		if line != expected {
			t.Fatalf("uid fetch response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a5 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesUIDFetchSetAfterSelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7:8 (FLAGS RFC822.SIZE)\r\n")); err != nil {
		t.Fatalf("write uid fetch set: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"a3 OK UID FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read uid fetch set response: %v", err)
		}
		if line != expected {
			t.Fatalf("uid fetch set response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesFetchSequenceSetAfterSelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 1:* (FLAGS RFC822.SIZE)\r\n")); err != nil {
		t.Fatalf("write fetch: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"a3 OK FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read fetch response: %v", err)
		}
		if line != expected {
			t.Fatalf("fetch response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesFetchEnvelopeAndInternalDate(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 1 (UID FLAGS INTERNALDATE ENVELOPE)\r\n")); err != nil {
		t.Fatalf("write fetch envelope: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 INTERNALDATE \"05-May-2026 12:34:56 +0900\" ENVELOPE (\"Tue, 05 May 2026 12:34:56 +0900\" \"Hello IMAP\" ((\"Sender\" NIL \"sender\" \"example.net\")) ((\"Sender\" NIL \"sender\" \"example.net\")) ((\"Sender\" NIL \"sender\" \"example.net\")) ((\"User\" NIL \"user\" \"example.com\")) NIL NIL NIL \"<message-7@example.net>\"))\r\n",
		"a3 OK FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read fetch envelope response: %v", err)
		}
		if line != expected {
			t.Fatalf("fetch envelope response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesFetchBodyStructure(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 1 (BODYSTRUCTURE)\r\n")); err != nil {
		t.Fatalf("write fetch bodystructure: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 BODYSTRUCTURE (\"TEXT\" \"PLAIN\" (\"CHARSET\" \"UTF-8\") NIL NIL \"7BIT\" 11 1 NIL NIL NIL NIL))\r\n",
		"a3 OK FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read bodystructure response: %v", err)
		}
		if line != expected {
			t.Fatalf("bodystructure response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesUIDFetchBodyAfterSelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7 BODY.PEEK[]\r\n")); err != nil {
		t.Fatalf("write uid fetch body: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read body literal header: %v", err)
	}
	if line != "* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 BODY[] {11}\r\n" {
		t.Fatalf("body literal header = %q", line)
	}
	body := make([]byte, 11)
	if _, err := io.ReadFull(reader, body); err != nil {
		t.Fatalf("read body literal: %v", err)
	}
	if string(body) != "hello world" {
		t.Fatalf("body = %q", body)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read body literal close: %v", err)
	}
	if line != ")\r\n" {
		t.Fatalf("literal close = %q", line)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read body fetch completion: %v", err)
	}
	if line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("completion = %q", line)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesUIDFetchHeaderAfterSelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 9 BODY.PEEK[HEADER]\r\n")); err != nil {
		t.Fatalf("write uid fetch header: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read header literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER] {37}\r\n" {
		t.Fatalf("header literal header = %q", line)
	}
	header := make([]byte, 37)
	if _, err := io.ReadFull(reader, header); err != nil {
		t.Fatalf("read header literal: %v", err)
	}
	if string(header) != "Subject: Hello\r\nFrom: sender@test\r\n\r\n" {
		t.Fatalf("header = %q", header)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("header close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesUIDFetchTextAfterSelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 9 RFC822.TEXT\r\n")); err != nil {
		t.Fatalf("write uid fetch text: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read text literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[TEXT] {17}\r\n" {
		t.Fatalf("text literal header = %q", line)
	}
	text := make([]byte, 17)
	if _, err := io.ReadFull(reader, text); err != nil {
		t.Fatalf("read text literal: %v", err)
	}
	if string(text) != "hello header body" {
		t.Fatalf("text = %q", text)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("text close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesUIDStoreAfterSelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID STORE 7:8 +FLAGS (\\Seen \\Flagged)\r\n")); err != nil {
		t.Fatalf("write uid store: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged))\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged))\r\n",
		"a3 OK UID STORE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read uid store response: %v", err)
		}
		if line != expected {
			t.Fatalf("uid store response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesUIDStoreSilentAfterSelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID STORE 7 +FLAGS.SILENT (\\Seen \\Flagged)\r\n")); err != nil {
		t.Fatalf("write uid store silent: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 OK UID STORE completed\r\n" {
		t.Fatalf("uid store silent completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestParseIMAPFieldsRejectsMalformedQuotedStrings(t *testing.T) {
	t.Parallel()

	if _, err := parseIMAPFields(`a1 LOGIN "user@example.com secret`); err == nil {
		t.Fatal("parseIMAPFields accepted unterminated quoted string")
	}
	if _, err := parseIMAPFields("a1 LOGIN \"user\nbad\" secret"); err == nil {
		t.Fatal("parseIMAPFields accepted quoted control character")
	}
	if _, err := parseIMAPFields("a1 LOGIN user@example.com {6}"); err == nil {
		t.Fatal("parseIMAPFields accepted unsupported literal")
	}
}

func TestDecodeSASLPlainRejectsMalformedResponses(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"",
		"not-base64",
		base64.StdEncoding.EncodeToString([]byte("user@example.com\x00secret")),
		base64.StdEncoding.EncodeToString([]byte("\x00\x00secret")),
	} {
		if username, password, ok := decodeSASLPlain(value); ok {
			t.Fatalf("decodeSASLPlain(%q) = %q %q true, want rejection", value, username, password)
		}
	}
}

func TestParseIMAPUIDSet(t *testing.T) {
	t.Parallel()

	got, ok := parseIMAPUIDSet("9:7,8,11")
	if !ok {
		t.Fatal("parseIMAPUIDSet rejected valid UID set")
	}
	want := []UID{7, 8, 9, 11}
	if len(got) != len(want) {
		t.Fatalf("UID set length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("UID set = %v, want %v", got, want)
		}
	}

	for _, value := range []string{"", "0", "7:*", "7:", "7:bad"} {
		if got, ok := parseIMAPUIDSet(value); ok {
			t.Fatalf("parseIMAPUIDSet(%q) = %v true, want rejection", value, got)
		}
	}
}

func TestParseIMAPSequenceSet(t *testing.T) {
	t.Parallel()

	got, ok := parseIMAPSequenceSet("2:*,1", 3)
	if !ok {
		t.Fatal("parseIMAPSequenceSet rejected valid sequence set")
	}
	want := []uint32{2, 3, 1}
	if len(got) != len(want) {
		t.Fatalf("sequence set length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sequence set = %v, want %v", got, want)
		}
	}

	for _, value := range []string{"", "0", "4", "1:4", "bad", "*"} {
		if got, ok := parseIMAPSequenceSet(value, 0); ok {
			t.Fatalf("parseIMAPSequenceSet(%q, 0) = %v true, want rejection", value, got)
		}
	}
}

func TestReadIMAPSectionLiteral(t *testing.T) {
	t.Parallel()

	header, err := readIMAPSectionLiteral(strings.NewReader("Subject: Hi\r\n\r\nbody"), true)
	if err != nil {
		t.Fatalf("readIMAPSectionLiteral header returned error: %v", err)
	}
	if string(header) != "Subject: Hi\r\n\r\n" {
		t.Fatalf("header = %q", header)
	}
	text, err := readIMAPSectionLiteral(strings.NewReader("Subject: Hi\r\n\r\nbody"), false)
	if err != nil {
		t.Fatalf("readIMAPSectionLiteral text returned error: %v", err)
	}
	if string(text) != "body" {
		t.Fatalf("text = %q", text)
	}
}

type fakeBackend struct{}

func (fakeBackend) Authenticate(context.Context, string, string) (Session, error) {
	return Session{UserID: "user-1", Username: "user@example.com"}, nil
}

func (fakeBackend) ListMailboxes(context.Context, ListMailboxesRequest) ([]Mailbox, error) {
	return []Mailbox{
		{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2},
		{ID: "archive", FullPath: "Archive\r\n2026", UIDValidity: 2, UIDNext: 3},
	}, nil
}

func (fakeBackend) GetMailbox(context.Context, UserID, MailboxID) (Mailbox, error) {
	return Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2, Unseen: 1}, nil
}

func (fakeBackend) ListMessages(context.Context, ListMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{
		{ID: "message-1", UID: 7, SequenceNumber: 1},
		{ID: "message-2", UID: 8, SequenceNumber: 2},
	}, nil
}

func (fakeBackend) FetchMessage(_ context.Context, req FetchMessageRequest) (Message, error) {
	internalDate := time.Date(2026, 5, 5, 12, 34, 56, 0, time.FixedZone("KST", 9*60*60))
	body := "hello world"
	size := int64(len(body))
	if req.UID == 9 {
		body = "Subject: Hello\r\nFrom: sender@test\r\n\r\nhello header body"
		size = int64(len(body))
	}
	return Message{
		Summary: MessageSummary{
			ID:             "message-1",
			UID:            req.UID,
			SequenceNumber: uint32(req.UID - 6),
			Envelope: Envelope{
				MessageID: "<message-7@example.net>",
				Subject:   "Hello IMAP",
				From:      []Address{{Name: "Sender", Mailbox: "sender", Host: "example.net"}},
				To:        []Address{{Name: "User", Mailbox: "user", Host: "example.com"}},
				Date:      internalDate,
			},
			Flags:        MessageFlags{Read: true, Starred: true},
			InternalDate: internalDate,
			Size:         size,
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}, nil
}

func (fakeBackend) StoreFlags(_ context.Context, req StoreFlagsRequest) ([]MessageSummary, error) {
	summaries := make([]MessageSummary, 0, len(req.UIDs))
	for _, uid := range req.UIDs {
		summaries = append(summaries, MessageSummary{ID: MessageID(fmt.Sprintf("message-%d", uid)), UID: uid, SequenceNumber: uint32(uid - 6), Flags: MessageFlags{Read: req.Flags.Read, Starred: req.Flags.Starred, Answered: req.Flags.Answered, Draft: req.Flags.Draft}})
	}
	return summaries, nil
}

func (fakeBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox:        Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2},
		PermanentFlags: []string{FlagSeen, FlagFlagged, FlagAnswered, FlagDraft},
	}, nil
}

func (fakeBackend) MoveMessages(context.Context, MoveMessagesRequest) ([]MessageSummary, error) {
	return nil, ErrUnsupportedMailboxMutation
}

func (fakeBackend) Expunge(context.Context, ExpungeRequest) ([]UID, error) {
	return nil, ErrUnsupportedMailboxMutation
}

func (fakeBackend) Subscribe(context.Context, UserID, MailboxID) (<-chan MailboxEvent, func(), error) {
	events := make(chan MailboxEvent)
	cancel := func() { close(events) }
	return events, cancel, nil
}

type eventBackend struct {
	fakeBackend
	events   chan MailboxEvent
	canceled bool
}

func (b *eventBackend) Subscribe(context.Context, UserID, MailboxID) (<-chan MailboxEvent, func(), error) {
	cancel := func() {
		b.canceled = true
	}
	return b.events, cancel, nil
}
