package pop3d

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

type mockMailbox struct {
	messages   []mockMessage
	deleted    map[int]bool
	contentErr map[int]error
}

type mockMessage struct {
	uidl    string
	size    int
	content string
}

func (m *mockMailbox) MessageCount() int {
	return len(m.messages)
}

func (m *mockMailbox) MessageSize(i int) int {
	if i < 0 || i >= len(m.messages) {
		return 0
	}
	if m.deleted[i] {
		return 0
	}
	return m.messages[i].size
}

func (m *mockMailbox) MessageUIDL(i int) string {
	if i < 0 || i >= len(m.messages) {
		return ""
	}
	return m.messages[i].uidl
}

func (m *mockMailbox) MessageContent(i int) string {
	if i < 0 || i >= len(m.messages) {
		return ""
	}
	return m.messages[i].content
}

func (m *mockMailbox) MessageContentWithError(i int) (string, error) {
	if err := m.contentErr[i]; err != nil {
		return "", err
	}
	if i < 0 || i >= len(m.messages) {
		return "", fmt.Errorf("invalid message index")
	}
	return m.messages[i].content, nil
}

func (m *mockMailbox) MarkDeleted(i int) error {
	if i < 0 || i >= len(m.messages) {
		return fmt.Errorf("invalid message index")
	}
	m.deleted[i] = true
	return nil
}

func (m *mockMailbox) ResetDeleted() {
	m.deleted = make(map[int]bool)
}

func (m *mockMailbox) Deleted(i int) bool {
	return m.deleted[i]
}

type mockStore struct {
	mailbox *mockMailbox
}

func (s *mockStore) Authenticate(user, pass string) (Mailbox, error) {
	if user != "alice" || pass != "secret" {
		return nil, fmt.Errorf("authentication failed")
	}
	return s.mailbox, nil
}

func newTestServer(t *testing.T) (*Server, net.Listener) {
	return newTestServerWithMessages(t, []mockMessage{
		{uidl: "msg001", size: 42, content: "From: a@example.com\r\n\r\nHello\r\n"},
		{uidl: "msg002", size: 38, content: "From: b@example.com\r\n\r\nWorld\r\n"},
	})
}

func newTestServerWithMessages(t *testing.T, messages []mockMessage) (*Server, net.Listener) {
	return newTestServerWithMessagesAndTLS(t, messages, nil)
}

func newTestServerWithMessagesAndTLS(t *testing.T, messages []mockMessage, tlsConfig *tls.Config) (*Server, net.Listener) {
	store := &mockStore{
		mailbox: &mockMailbox{
			messages: messages,
			deleted:  make(map[int]bool),
		},
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := &Server{
		Store:       store,
		TLSConfig:   tlsConfig,
		Greeting:    "gogomail POP3 ready",
		IdleTimeout: 5 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && !strings.Contains(err.Error(), "closed") {
			t.Logf("server error: %v", err)
		}
	}()

	return server, listener
}

func pop3Login(t *testing.T, tp *textproto.Conn) {
	t.Helper()
	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
}

func pop3ReadDotLines(t *testing.T, tp *textproto.Conn) string {
	t.Helper()
	data, err := io.ReadAll(tp.DotReader())
	if err != nil {
		t.Fatalf("read dot response: %v", err)
	}
	return string(data)
}

func pop3Conn(t *testing.T, addr string) *textproto.Conn {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	tp := textproto.NewConn(conn)
	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("greeting: %v", err)
	}
	if !strings.HasPrefix(line, "+OK") {
		t.Fatalf("unexpected greeting: %s", line)
	}
	return tp
}

func pop3Cmd(t *testing.T, tp *textproto.Conn, expected string, format string, args ...interface{}) string {
	t.Helper()
	id, err := tp.Cmd(format, args...)
	if err != nil {
		t.Fatalf("cmd: %v", err)
	}
	tp.StartResponse(id)
	defer tp.EndResponse(id)

	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("read line: %v", err)
	}
	if !strings.HasPrefix(line, expected) {
		t.Fatalf("expected %s, got: %s", expected, line)
	}
	return line
}

func TestPOP3Greeting(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	pop3Conn(t, listener.Addr().String())
}

func TestPOP3AuthAndStat(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	line := pop3Cmd(t, tp, "+OK", "STAT")
	if !strings.Contains(line, "2 ") {
		t.Fatalf("expected 2 messages in STAT, got: %s", line)
	}
}

func TestPOP3AuthFail(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER bob")
	pop3Cmd(t, tp, "-ERR", "PASS wrong")
}

func TestPOP3RejectsConcurrentMaildropAccess(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	first := pop3Conn(t, listener.Addr().String())
	defer first.Close()
	pop3Login(t, first)

	second := pop3Conn(t, listener.Addr().String())
	defer second.Close()
	pop3Cmd(t, second, "+OK", "USER alice")
	pop3Cmd(t, second, "-ERR", "PASS secret")

	pop3Cmd(t, first, "+OK", "STAT")
	pop3Cmd(t, first, "+OK", "QUIT")

	third := pop3Conn(t, listener.Addr().String())
	defer third.Close()
	pop3Login(t, third)
}

func TestPOP3ReleasesMaildropLockOnConnectionClose(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	first := pop3Conn(t, listener.Addr().String())
	pop3Login(t, first)
	first.Close()

	deadline := time.Now().Add(time.Second)
	for {
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		tp := textproto.NewConn(conn)
		if _, err := tp.ReadLine(); err != nil {
			t.Fatalf("greeting: %v", err)
		}
		pop3Cmd(t, tp, "+OK", "USER alice")
		line := pop3Cmd(t, tp, "", "PASS secret")
		if strings.HasPrefix(line, "+OK") {
			tp.Close()
			break
		}
		tp.Close()
		if time.Now().After(deadline) {
			t.Fatalf("maildrop lock was not released after connection close; last response: %s", line)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPOP3RejectsConnectionsOverLimit(t *testing.T) {
	store := &mockStore{
		mailbox: &mockMailbox{
			messages: []mockMessage{{uidl: "msg001", size: 42, content: "Hello\r\n"}},
			deleted:  make(map[int]bool),
		},
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	server := &Server{
		Store:          store,
		Greeting:       "test",
		IdleTimeout:    5 * time.Second,
		MaxConnections: 1,
	}
	go func() { _ = server.Serve(listener) }()

	first := pop3Conn(t, listener.Addr().String())
	defer first.Close()

	secondConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial second: %v", err)
	}
	second := textproto.NewConn(secondConn)
	line, err := second.ReadLine()
	if err != nil {
		t.Fatalf("read connection-limit rejection: %v", err)
	}
	if !strings.HasPrefix(line, "-ERR") {
		t.Fatalf("expected -ERR connection-limit rejection, got %s", line)
	}
	second.Close()
}

func TestPOP3ConnectionLimitSlotReleasedOnClose(t *testing.T) {
	store := &mockStore{
		mailbox: &mockMailbox{
			messages: []mockMessage{{uidl: "msg001", size: 42, content: "Hello\r\n"}},
			deleted:  make(map[int]bool),
		},
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	server := &Server{
		Store:          store,
		Greeting:       "test",
		IdleTimeout:    5 * time.Second,
		MaxConnections: 1,
	}
	go func() { _ = server.Serve(listener) }()

	first := pop3Conn(t, listener.Addr().String())
	first.Close()

	deadline := time.Now().Add(time.Second)
	for {
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		tp := textproto.NewConn(conn)
		line, err := tp.ReadLine()
		tp.Close()
		if err == nil && strings.HasPrefix(line, "+OK") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("connection slot was not released; last line=%q err=%v", line, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPOP3List(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")

	pop3Cmd(t, tp, "+OK", "LIST")
	reader := tp.DotReader()
	scanner := bufio.NewScanner(reader)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines in LIST, got %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "1 ") {
		t.Fatalf("expected first message line, got: %s", lines[0])
	}
}

func TestPOP3Retr(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "+OK", "RETR 1")

	reader := tp.DotReader()
	data := make([]byte, 1024)
	n, _ := reader.Read(data)
	if n == 0 {
		t.Fatalf("expected message content")
	}
	content := string(data[:n])
	if !strings.Contains(content, "From: a@example.com") {
		t.Fatalf("expected message content, got: %s", content)
	}
}

func TestPOP3RetrDotStuffsMessageBody(t *testing.T) {
	_, listener := newTestServerWithMessages(t, []mockMessage{{
		uidl:    "msg001",
		size:    64,
		content: "From: a@example.com\r\n\r\nfirst\r\n.secret\r\n.\r\nlast\r\n",
	}})
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "RETR 1")

	data, err := io.ReadAll(tp.DotReader())
	if err != nil {
		t.Fatalf("read RETR body: %v", err)
	}
	content := string(data)
	for _, want := range []string{"first", ".secret", "\n.\n", "last"} {
		if !strings.Contains(content, want) {
			t.Fatalf("RETR body = %q, want %q", content, want)
		}
	}
}

func TestPOP3RetrReturnsErrorWhenMessageContentFetchFails(t *testing.T) {
	server, listener := newTestServerWithMessages(t, []mockMessage{{
		uidl: "msg001",
		size: 64,
	}})
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Login(t, tp)
	server.Store.(*mockStore).mailbox.contentErr = map[int]error{0: fmt.Errorf("object storage read failed")}
	pop3Cmd(t, tp, "-ERR", "RETR 1")
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3DeleAndRset(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "+OK", "DELE 1")
	pop3Cmd(t, tp, "+OK", "RSET")

	line := pop3Cmd(t, tp, "+OK", "STAT")
	if !strings.Contains(line, "2 ") {
		t.Fatalf("expected 2 messages after RSET, got: %s", line)
	}
}

func TestPOP3Uidl(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "+OK", "UIDL")

	reader := tp.DotReader()
	scanner := bufio.NewScanner(reader)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 UIDL lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "msg001") {
		t.Fatalf("expected msg001 in UIDL, got: %s", lines[0])
	}
}

func TestPOP3Top(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "+OK", "TOP 1 0")

	reader := tp.DotReader()
	data := make([]byte, 1024)
	n, _ := reader.Read(data)
	if n == 0 {
		t.Fatalf("expected headers from TOP")
	}
}

func TestPOP3TopDotStuffsHeaderAndBody(t *testing.T) {
	_, listener := newTestServerWithMessages(t, []mockMessage{{
		uidl:    "msg001",
		size:    96,
		content: "From: a@example.com\r\n.X-Debug: header\r\n\r\n.body\r\n.\r\nlast\r\n",
	}})
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "TOP 1 2")

	data, err := io.ReadAll(tp.DotReader())
	if err != nil {
		t.Fatalf("read TOP body: %v", err)
	}
	content := string(data)
	for _, want := range []string{".X-Debug: header", ".body", "\n.\n"} {
		if !strings.Contains(content, want) {
			t.Fatalf("TOP body = %q, want %q", content, want)
		}
	}
	if strings.Contains(content, "last") {
		t.Fatalf("TOP returned too many body lines: %q", content)
	}
}

func TestPOP3TopReturnsErrorWhenMessageContentFetchFails(t *testing.T) {
	server, listener := newTestServerWithMessages(t, []mockMessage{{
		uidl: "msg001",
		size: 64,
	}})
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Login(t, tp)
	server.Store.(*mockStore).mailbox.contentErr = map[int]error{0: fmt.Errorf("object storage read failed")}
	pop3Cmd(t, tp, "-ERR", "TOP 1 1")
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3Noop(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "+OK", "NOOP")
}

func TestPOP3Quit(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "QUIT")
}

func TestPOP3Capa(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "CAPA")
	reader := tp.DotReader()
	data := make([]byte, 1024)
	n, _ := reader.Read(data)
	if n == 0 {
		t.Fatalf("expected CAPA list")
	}
	capa := string(data[:n])
	if !strings.Contains(capa, "UIDL") {
		t.Fatalf("expected UIDL in CAPA, got: %s", capa)
	}
}

func TestPOP3CapaDoesNotAdvertiseSTLSWithoutTLSConfig(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "CAPA")
	capa := pop3ReadDotLines(t, tp)
	if strings.Contains(capa, "STLS") {
		t.Fatalf("CAPA advertised STLS without TLS config: %s", capa)
	}
	pop3Cmd(t, tp, "-ERR", "STLS")
}

func TestPOP3CapaAdvertisesSTLSOnlyBeforeAuthentication(t *testing.T) {
	_, listener := newTestServerWithMessagesAndTLS(t, []mockMessage{
		{uidl: "msg001", size: 42, content: "From: a@example.com\r\n\r\nHello\r\n"},
	}, &tls.Config{MinVersion: tls.VersionTLS12})
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "CAPA")
	capa := pop3ReadDotLines(t, tp)
	if !strings.Contains(capa, "STLS") {
		t.Fatalf("CAPA did not advertise STLS before auth: %s", capa)
	}

	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "CAPA")
	capa = pop3ReadDotLines(t, tp)
	if strings.Contains(capa, "STLS") {
		t.Fatalf("transaction CAPA advertised STLS after auth: %s", capa)
	}
	pop3Cmd(t, tp, "-ERR", "STLS")
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3TransactionQuitAppliesDele(t *testing.T) {
	store := &mockStore{
		mailbox: &mockMailbox{
			messages: []mockMessage{
				{uidl: "msg001", size: 42, content: "Hello\r\n"},
			},
			deleted: make(map[int]bool),
		},
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	server := &Server{
		Store:       store,
		Greeting:    "test",
		IdleTimeout: 5 * time.Second,
	}
	go server.Serve(listener)

	tp := pop3Conn(t, listener.Addr().String())
	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "+OK", "DELE 1")
	pop3Cmd(t, tp, "+OK", "QUIT")

	if !store.mailbox.Deleted(0) {
		t.Fatalf("expected message to be marked deleted after QUIT")
	}
}

func TestPOP3InvalidCommand(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "-ERR", "INVALIDCMD")
}

func TestPOP3ListSingle(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	line := pop3Cmd(t, tp, "+OK", "LIST 1")
	if !strings.Contains(line, "42") {
		t.Fatalf("expected size 42 in LIST 1, got: %s", line)
	}
}

func TestPOP3RetrInvalid(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "-ERR", "RETR 99")
}

// commitMailbox wraps mockMailbox and adds a CommitDeletes method.
type commitMailbox struct {
	*mockMailbox
	commitErr error
}

func (c *commitMailbox) CommitDeletes() error {
	return c.commitErr
}

// commitStore is a Store that returns a commitMailbox.
type commitStore struct {
	mailbox *commitMailbox
}

func (s *commitStore) Authenticate(user, pass string) (Mailbox, error) {
	if user != "alice" || pass != "secret" {
		return nil, fmt.Errorf("authentication failed")
	}
	return s.mailbox, nil
}

func newCommitServer(t *testing.T, mb *commitMailbox) (*Server, net.Listener) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &Server{
		Store:       &commitStore{mailbox: mb},
		Greeting:    "test",
		IdleTimeout: 5 * time.Second,
	}
	go func() { _ = server.Serve(listener) }()
	return server, listener
}

// TestPOP3CommitDeletesErrorRollsBack verifies that when CommitDeletes returns
// an error on QUIT, the server sends -ERR and rolls back the deletion marks.
func TestPOP3CommitDeletesErrorRollsBack(t *testing.T) {
	mb := &commitMailbox{
		mockMailbox: &mockMailbox{
			messages: []mockMessage{
				{uidl: "msg001", size: 42, content: "Hello\r\n"},
			},
			deleted: make(map[int]bool),
		},
		commitErr: fmt.Errorf("db write failed"),
	}
	_, listener := newCommitServer(t, mb)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "+OK", "DELE 1")
	// QUIT must return -ERR because CommitDeletes fails.
	pop3Cmd(t, tp, "-ERR", "QUIT")

	// After rollback, the message must no longer be marked deleted.
	if mb.mockMailbox.Deleted(0) {
		t.Fatal("expected deletion to be rolled back after CommitDeletes failure")
	}
}

// TestPOP3CommitDeletesSuccess verifies that a successful CommitDeletes on QUIT
// returns +OK to the client.
func TestPOP3CommitDeletesSuccess(t *testing.T) {
	mb := &commitMailbox{
		mockMailbox: &mockMailbox{
			messages: []mockMessage{
				{uidl: "msg001", size: 42, content: "Hello\r\n"},
			},
			deleted: make(map[int]bool),
		},
		commitErr: nil,
	}
	_, listener := newCommitServer(t, mb)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "+OK", "DELE 1")
	pop3Cmd(t, tp, "+OK", "QUIT")
}
