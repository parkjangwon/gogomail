package pop3d

import (
	"bufio"
	"fmt"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

type mockMailbox struct {
	messages []mockMessage
	deleted  map[int]bool
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
	store := &mockStore{
		mailbox: &mockMailbox{
			messages: []mockMessage{
				{uidl: "msg001", size: 42, content: "From: a@example.com\r\n\r\nHello\r\n"},
				{uidl: "msg002", size: 38, content: "From: b@example.com\r\n\r\nWorld\r\n"},
			},
			deleted: make(map[int]bool),
		},
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := &Server{
		Store:       store,
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
