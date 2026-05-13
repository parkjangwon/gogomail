package pop3d

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
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

func testPOP3TLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate TLS key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create TLS certificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("parse TLS key pair: %v", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
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

func pop3Capa(t *testing.T, tp *textproto.Conn) map[string]bool {
	t.Helper()
	pop3Cmd(t, tp, "+OK", "CAPA")
	data := pop3ReadDotLines(t, tp)
	lines := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if line != "" {
			lines[line] = true
		}
	}
	return lines
}

func assertPOP3AuthCapabilities(t *testing.T, tp *textproto.Conn, context string) {
	t.Helper()
	capa := pop3Capa(t, tp)
	if !capa["USER"] || !capa["SASL PLAIN LOGIN"] {
		t.Fatalf("expected auth capabilities after %s, got: %#v", context, capa)
	}
}

func assertPOP3AuthenticatedState(t *testing.T, tp *textproto.Conn, context string, wantCount string) {
	t.Helper()
	capa := pop3Capa(t, tp)
	if capa["USER"] || capa["SASL PLAIN LOGIN"] {
		t.Fatalf("transaction CAPA advertised auth capabilities after %s: %#v", context, capa)
	}
	line := pop3Cmd(t, tp, "+OK", "STAT")
	if !strings.Contains(line, wantCount+" ") {
		t.Fatalf("expected authenticated STAT after %s, got: %s", context, line)
	}
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

func pop3ConnWithDeadline(t *testing.T, addr string, timeout time.Duration) *textproto.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		conn.Close()
		t.Fatalf("set deadline: %v", err)
	}
	tp := textproto.NewConn(conn)
	line, err := tp.ReadLine()
	if err != nil {
		tp.Close()
		t.Fatalf("greeting: %v", err)
	}
	if !strings.HasPrefix(line, "+OK") {
		tp.Close()
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

func pop3BeginAuth(t *testing.T, tp *textproto.Conn, command string) uint {
	t.Helper()
	id, err := tp.Cmd("%s", command)
	if err != nil {
		t.Fatalf("cmd: %v", err)
	}
	tp.StartResponse(id)
	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth continuation: %v", err)
	}
	if !strings.HasPrefix(line, "+") {
		t.Fatalf("expected auth continuation, got: %s", line)
	}
	return id
}

func pop3CancelAuth(t *testing.T, tp *textproto.Conn, id uint) {
	t.Helper()
	if err := tp.PrintfLine("*"); err != nil {
		t.Fatalf("send auth cancellation: %v", err)
	}
	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth cancellation response: %v", err)
	}
	if !strings.HasPrefix(line, "-ERR authentication cancelled") {
		t.Fatalf("expected authentication cancelled, got: %s", line)
	}
	tp.EndResponse(id)
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

func TestPOP3UserPassAuthenticationUpdatesCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Login(t, tp)
	assertPOP3AuthenticatedState(t, tp, "USER/PASS", "2")
}

func TestPOP3AuthFail(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER bob")
	pop3Cmd(t, tp, "-ERR", "PASS wrong")
}

func TestPOP3UserPassFailureKeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	line := pop3Cmd(t, tp, "-ERR", "PASS wrong")
	if !strings.Contains(line, "authentication failed") {
		t.Fatalf("expected authentication failure, got: %s", line)
	}
	assertPOP3AuthCapabilities(t, tp, "USER/PASS failure")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3PassWithoutUserKeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	line := pop3Cmd(t, tp, "-ERR", "PASS secret")
	if !strings.Contains(line, "authentication failed") {
		t.Fatalf("expected authentication failure, got: %s", line)
	}
	assertPOP3AuthCapabilities(t, tp, "PASS without USER")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3UserCanBeReplacedBeforePass(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER bob")
	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	assertPOP3AuthenticatedState(t, tp, "replaced USER/PASS", "2")
}

func TestPOP3UserSyntaxErrorKeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	line := pop3Cmd(t, tp, "-ERR", "USER alice extra")
	if !strings.Contains(line, "syntax error") {
		t.Fatalf("expected syntax error, got: %s", line)
	}
	assertPOP3AuthCapabilities(t, tp, "USER syntax error")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3PassSyntaxErrorKeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	line := pop3Cmd(t, tp, "-ERR", "PASS secret extra")
	if !strings.Contains(line, "syntax error") {
		t.Fatalf("expected syntax error, got: %s", line)
	}
	assertPOP3AuthCapabilities(t, tp, "PASS syntax error")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	assertPOP3AuthenticatedState(t, tp, "PASS after syntax error", "2")
}

func TestPOP3AuthorizationUnknownCommandKeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	line := pop3Cmd(t, tp, "-ERR", "BOGUS")
	if !strings.Contains(line, "unknown command") {
		t.Fatalf("expected unknown command response, got: %s", line)
	}
	assertPOP3AuthCapabilities(t, tp, "authorization unknown command")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthorizationEmptyCommandKeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	line := pop3Cmd(t, tp, "-ERR", "")
	if !strings.Contains(line, "syntax error") {
		t.Fatalf("expected syntax error response, got: %s", line)
	}
	assertPOP3AuthCapabilities(t, tp, "authorization empty command")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3TransactionUnknownCommandKeepsSessionUsable(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Login(t, tp)
	line := pop3Cmd(t, tp, "-ERR", "BOGUS")
	if !strings.Contains(line, "unknown command") {
		t.Fatalf("expected unknown command response, got: %s", line)
	}
	pop3Cmd(t, tp, "+OK", "NOOP")
	line = pop3Cmd(t, tp, "+OK", "STAT")
	if !strings.Contains(line, "2 ") {
		t.Fatalf("expected transaction session to remain usable, got: %s", line)
	}
}

func TestPOP3TransactionEmptyCommandKeepsSessionUsable(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Login(t, tp)
	line := pop3Cmd(t, tp, "-ERR", "")
	if !strings.Contains(line, "syntax error") {
		t.Fatalf("expected syntax error response, got: %s", line)
	}
	pop3Cmd(t, tp, "+OK", "NOOP")
	line = pop3Cmd(t, tp, "+OK", "STAT")
	if !strings.Contains(line, "2 ") {
		t.Fatalf("expected transaction session to remain usable, got: %s", line)
	}
}

func TestPOP3TransactionRejectsUserAndPass(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Login(t, tp)
	for _, cmd := range []string{"USER alice", "PASS secret"} {
		line := pop3Cmd(t, tp, "-ERR", "%s", cmd)
		if !strings.Contains(line, "unknown command") {
			t.Fatalf("expected unknown command for %s, got: %s", cmd, line)
		}
	}
	pop3Cmd(t, tp, "+OK", "NOOP")
	line := pop3Cmd(t, tp, "+OK", "STAT")
	if !strings.Contains(line, "2 ") {
		t.Fatalf("expected transaction session to remain usable, got: %s", line)
	}
}

func TestPOP3TransactionRejectsAuth(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Login(t, tp)
	for _, cmd := range []string{"AUTH PLAIN", "AUTH LOGIN"} {
		line := pop3Cmd(t, tp, "-ERR", "%s", cmd)
		if !strings.Contains(line, "unknown command") {
			t.Fatalf("expected unknown command for %s, got: %s", cmd, line)
		}
	}
	pop3Cmd(t, tp, "+OK", "NOOP")
	line := pop3Cmd(t, tp, "+OK", "STAT")
	if !strings.Contains(line, "2 ") {
		t.Fatalf("expected transaction session to remain usable, got: %s", line)
	}
}

func TestPOP3AuthPlainRejectsExtraArguments(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	initial := base64.StdEncoding.EncodeToString([]byte("\x00alice\x00secret"))
	pop3Cmd(t, tp, "-ERR", "AUTH PLAIN %s ignored", initial)
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthPlainInitialResponseAuthenticates(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	initial := base64.StdEncoding.EncodeToString([]byte("\x00alice\x00secret"))
	pop3Cmd(t, tp, "+OK", "AUTH PLAIN %s", initial)
	assertPOP3AuthenticatedState(t, tp, "AUTH PLAIN initial response", "2")
}

func TestPOP3AuthPlainInitialResponseWrongPasswordKeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	initial := base64.StdEncoding.EncodeToString([]byte("\x00alice\x00wrong"))
	line := pop3Cmd(t, tp, "-ERR", "AUTH PLAIN %s", initial)
	if !strings.Contains(line, "authentication failed") {
		t.Fatalf("expected authentication failure, got: %s", line)
	}
	assertPOP3AuthCapabilities(t, tp, "AUTH PLAIN initial response wrong password")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthPlainInvalidBase64KeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	line := pop3Cmd(t, tp, "-ERR", "AUTH PLAIN not-base64!")
	if !strings.Contains(line, "invalid base64") {
		t.Fatalf("expected invalid base64 response, got: %s", line)
	}
	assertPOP3AuthCapabilities(t, tp, "AUTH PLAIN invalid base64")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthPlainChallengeInvalidBase64KeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	id := pop3BeginAuth(t, tp, "AUTH PLAIN")
	if err := tp.PrintfLine("not-base64!"); err != nil {
		t.Fatalf("send invalid auth plain response: %v", err)
	}
	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth plain error: %v", err)
	}
	if !strings.HasPrefix(line, "-ERR invalid base64") {
		t.Fatalf("expected invalid base64 response, got: %s", line)
	}
	tp.EndResponse(id)

	assertPOP3AuthCapabilities(t, tp, "AUTH PLAIN challenge invalid base64")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthPlainChallengeInvalidFormatKeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	id := pop3BeginAuth(t, tp, "AUTH PLAIN")
	invalid := base64.StdEncoding.EncodeToString([]byte("alice\x00secret"))
	if err := tp.PrintfLine("%s", invalid); err != nil {
		t.Fatalf("send invalid auth plain response: %v", err)
	}
	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth plain error: %v", err)
	}
	if !strings.HasPrefix(line, "-ERR invalid credentials format") {
		t.Fatalf("expected invalid credentials format response, got: %s", line)
	}
	tp.EndResponse(id)

	assertPOP3AuthCapabilities(t, tp, "AUTH PLAIN challenge invalid format")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthPlainChallengeAuthenticates(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	id := pop3BeginAuth(t, tp, "AUTH PLAIN")
	credential := base64.StdEncoding.EncodeToString([]byte("\x00alice\x00secret"))
	if err := tp.PrintfLine("%s", credential); err != nil {
		t.Fatalf("send auth plain response: %v", err)
	}
	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth plain success: %v", err)
	}
	if !strings.HasPrefix(line, "+OK") {
		t.Fatalf("expected auth plain success, got: %s", line)
	}
	tp.EndResponse(id)

	assertPOP3AuthenticatedState(t, tp, "AUTH PLAIN challenge", "2")
}

func TestPOP3AuthPlainChallengeWrongPasswordKeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	id := pop3BeginAuth(t, tp, "AUTH PLAIN")
	credential := base64.StdEncoding.EncodeToString([]byte("\x00alice\x00wrong"))
	if err := tp.PrintfLine("%s", credential); err != nil {
		t.Fatalf("send auth plain response: %v", err)
	}
	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth plain failure: %v", err)
	}
	if !strings.HasPrefix(line, "-ERR authentication failed") {
		t.Fatalf("expected auth plain failure, got: %s", line)
	}
	tp.EndResponse(id)

	assertPOP3AuthCapabilities(t, tp, "AUTH PLAIN challenge wrong password")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthPlainInvalidFormatKeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	invalid := base64.StdEncoding.EncodeToString([]byte("alice\x00secret"))
	line := pop3Cmd(t, tp, "-ERR", "AUTH PLAIN %s", invalid)
	if !strings.Contains(line, "invalid credentials format") {
		t.Fatalf("expected invalid credentials format response, got: %s", line)
	}
	assertPOP3AuthCapabilities(t, tp, "AUTH PLAIN invalid format")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthLoginRejectsExtraArguments(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "-ERR", "AUTH LOGIN ignored")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthPlainCancellationKeepsSessionUsable(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	id := pop3BeginAuth(t, tp, "AUTH PLAIN")
	pop3CancelAuth(t, tp, id)
	assertPOP3AuthCapabilities(t, tp, "AUTH PLAIN cancellation")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthLoginUsernameCancellationKeepsSessionUsable(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	id := pop3BeginAuth(t, tp, "AUTH LOGIN")
	pop3CancelAuth(t, tp, id)
	assertPOP3AuthCapabilities(t, tp, "AUTH LOGIN username cancellation")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthLoginInvalidUsernameBase64KeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	id := pop3BeginAuth(t, tp, "AUTH LOGIN")
	if err := tp.PrintfLine("not-base64!"); err != nil {
		t.Fatalf("send invalid auth login username: %v", err)
	}
	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth login username error: %v", err)
	}
	if !strings.HasPrefix(line, "-ERR invalid base64") {
		t.Fatalf("expected invalid base64 response, got: %s", line)
	}
	tp.EndResponse(id)

	assertPOP3AuthCapabilities(t, tp, "AUTH LOGIN username invalid base64")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthLoginPasswordCancellationKeepsSessionUsable(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	id := pop3BeginAuth(t, tp, "AUTH LOGIN")
	if err := tp.PrintfLine("%s", base64.StdEncoding.EncodeToString([]byte("alice"))); err != nil {
		t.Fatalf("send auth login username: %v", err)
	}
	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth login password continuation: %v", err)
	}
	if !strings.HasPrefix(line, "+") {
		t.Fatalf("expected password continuation, got: %s", line)
	}
	pop3CancelAuth(t, tp, id)
	assertPOP3AuthCapabilities(t, tp, "AUTH LOGIN password cancellation")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthLoginAuthenticates(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	id := pop3BeginAuth(t, tp, "AUTH LOGIN")
	if err := tp.PrintfLine("%s", base64.StdEncoding.EncodeToString([]byte("alice"))); err != nil {
		t.Fatalf("send auth login username: %v", err)
	}
	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth login password continuation: %v", err)
	}
	if !strings.HasPrefix(line, "+") {
		t.Fatalf("expected password continuation, got: %s", line)
	}
	if err := tp.PrintfLine("%s", base64.StdEncoding.EncodeToString([]byte("secret"))); err != nil {
		t.Fatalf("send auth login password: %v", err)
	}
	line, err = tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth login success: %v", err)
	}
	if !strings.HasPrefix(line, "+OK") {
		t.Fatalf("expected auth login success, got: %s", line)
	}
	tp.EndResponse(id)

	assertPOP3AuthenticatedState(t, tp, "AUTH LOGIN", "2")
}

func TestPOP3AuthLoginWrongPasswordKeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	id := pop3BeginAuth(t, tp, "AUTH LOGIN")
	if err := tp.PrintfLine("%s", base64.StdEncoding.EncodeToString([]byte("alice"))); err != nil {
		t.Fatalf("send auth login username: %v", err)
	}
	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth login password continuation: %v", err)
	}
	if !strings.HasPrefix(line, "+") {
		t.Fatalf("expected password continuation, got: %s", line)
	}
	if err := tp.PrintfLine("%s", base64.StdEncoding.EncodeToString([]byte("wrong"))); err != nil {
		t.Fatalf("send auth login password: %v", err)
	}
	line, err = tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth login failure: %v", err)
	}
	if !strings.HasPrefix(line, "-ERR authentication failed") {
		t.Fatalf("expected auth login failure, got: %s", line)
	}
	tp.EndResponse(id)

	assertPOP3AuthCapabilities(t, tp, "AUTH LOGIN wrong password")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3AuthLoginInvalidPasswordBase64KeepsAuthCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	id := pop3BeginAuth(t, tp, "AUTH LOGIN")
	if err := tp.PrintfLine("%s", base64.StdEncoding.EncodeToString([]byte("alice"))); err != nil {
		t.Fatalf("send auth login username: %v", err)
	}
	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth login password continuation: %v", err)
	}
	if !strings.HasPrefix(line, "+") {
		t.Fatalf("expected password continuation, got: %s", line)
	}
	if err := tp.PrintfLine("not-base64!"); err != nil {
		t.Fatalf("send invalid auth login password: %v", err)
	}
	line, err = tp.ReadLine()
	if err != nil {
		t.Fatalf("read auth login password error: %v", err)
	}
	if !strings.HasPrefix(line, "-ERR invalid base64") {
		t.Fatalf("expected invalid base64 response, got: %s", line)
	}
	tp.EndResponse(id)

	assertPOP3AuthCapabilities(t, tp, "AUTH LOGIN password invalid base64")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
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

func TestPOP3RetrAnnouncesListSize(t *testing.T) {
	_, listener := newTestServerWithMessages(t, []mockMessage{{
		uidl:    "msg001",
		size:    42,
		content: "From: a@example.com\n\nHello with LF endings\n",
	}})
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Login(t, tp)
	listLine := pop3Cmd(t, tp, "+OK", "LIST 1")
	retrLine := pop3Cmd(t, tp, "+OK", "RETR 1")
	_, _ = io.Copy(io.Discard, tp.DotReader())

	if !strings.Contains(listLine, "1 42") {
		t.Fatalf("LIST response = %q, want stored size 42", listLine)
	}
	if !strings.Contains(retrLine, "42 octets") {
		t.Fatalf("RETR response = %q, want LIST size in octets", retrLine)
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

func TestPOP3RsetRestoresWireVisibility(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "+OK", "DELE 1")
	pop3Cmd(t, tp, "+OK", "RSET")

	if line := pop3Cmd(t, tp, "+OK", "LIST 1"); !strings.Contains(line, "1 42") {
		t.Fatalf("expected LIST 1 after RSET to restore message size, got: %s", line)
	}
	if line := pop3Cmd(t, tp, "+OK", "UIDL 1"); !strings.Contains(line, "msg001") {
		t.Fatalf("expected UIDL 1 after RSET to restore message UIDL, got: %s", line)
	}

	pop3Cmd(t, tp, "+OK", "RETR 1")
	data, err := io.ReadAll(tp.DotReader())
	if err != nil {
		t.Fatalf("read RETR body after RSET: %v", err)
	}
	if !strings.Contains(string(data), "Hello") {
		t.Fatalf("expected RETR 1 body after RSET, got %q", string(data))
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

func TestPOP3UidlHidesDeletedMessages(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "+OK", "DELE 1")
	pop3Cmd(t, tp, "-ERR", "UIDL 1")
	pop3Cmd(t, tp, "+OK", "UIDL")

	reader := tp.DotReader()
	scanner := bufio.NewScanner(reader)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) != 1 {
		t.Fatalf("expected 1 UIDL line after delete, got %d", len(lines))
	}
	if strings.Contains(lines[0], "msg001") {
		t.Fatalf("deleted message UIDL should be hidden, got: %s", lines[0])
	}
	if !strings.Contains(lines[0], "msg002") {
		t.Fatalf("expected remaining msg002 UIDL, got: %s", lines[0])
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

func TestPOP3AuthStateQuitClosesConnection(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3ConnWithDeadline(t, listener.Addr().String(), 2*time.Second)
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "QUIT")
	if line, err := tp.ReadLine(); err == nil {
		t.Fatalf("expected connection close after auth-state QUIT, got line: %s", line)
	}
}

func TestPOP3Capa(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	capa := pop3Capa(t, tp)
	for _, want := range []string{"IMPLEMENTATION gogomail", "LOGIN-DELAY 0", "UIDL", "TOP", "USER", "SASL PLAIN LOGIN"} {
		if !capa[want] {
			t.Fatalf("expected CAPA %q in %#v", want, capa)
		}
	}
}

func TestPOP3TransactionCapaOmitsAuthOnlyCapabilities(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Login(t, tp)
	capa := pop3Capa(t, tp)
	for _, want := range []string{"IMPLEMENTATION gogomail", "LOGIN-DELAY 0", "UIDL", "TOP"} {
		if !capa[want] {
			t.Fatalf("expected transaction CAPA %q in %#v", want, capa)
		}
	}
	for _, unwanted := range []string{"USER", "SASL PLAIN LOGIN", "STLS"} {
		if capa[unwanted] {
			t.Fatalf("transaction CAPA advertised auth-only capability %q in %#v", unwanted, capa)
		}
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

func TestPOP3STLSUnavailableInAuthStateKeepsSessionUsable(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	line := pop3Cmd(t, tp, "-ERR", "STLS")
	if !strings.Contains(line, "STLS not available") {
		t.Fatalf("expected auth-state STLS unavailable denial, got: %s", line)
	}
	pop3Login(t, tp)
	line = pop3Cmd(t, tp, "+OK", "STAT")
	if !strings.Contains(line, "2 ") {
		t.Fatalf("expected session to remain usable after STLS denial, got: %s", line)
	}
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

func TestPOP3STLSDeniedInTransactionStateKeepsSessionUsable(t *testing.T) {
	_, listener := newTestServerWithMessagesAndTLS(t, []mockMessage{
		{uidl: "msg001", size: 42, content: "From: a@example.com\r\n\r\nHello\r\n"},
	}, testPOP3TLSConfig(t))
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Login(t, tp)
	line := pop3Cmd(t, tp, "-ERR", "STLS")
	if !strings.Contains(line, "STLS not available in transaction state") {
		t.Fatalf("expected transaction-state STLS denial, got: %s", line)
	}
	pop3Cmd(t, tp, "+OK", "NOOP")
	line = pop3Cmd(t, tp, "+OK", "STAT")
	if !strings.Contains(line, "1 ") {
		t.Fatalf("expected transaction session to remain usable, got: %s", line)
	}
}

func TestPOP3STLSResetsPreTLSUserState(t *testing.T) {
	_, listener := newTestServerWithMessagesAndTLS(t, []mockMessage{
		{uidl: "msg001", size: 42, content: "From: a@example.com\r\n\r\nHello\r\n"},
	}, testPOP3TLSConfig(t))
	defer listener.Close()

	conn, err := net.Dial("tcp", listener.Addr().String())
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
	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "STLS")

	tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
	if err := tlsConn.Handshake(); err != nil {
		t.Fatalf("TLS handshake: %v", err)
	}
	tp = textproto.NewConn(tlsConn)
	defer tp.Close()

	pop3Cmd(t, tp, "-ERR", "PASS secret")
	pop3Login(t, tp)
	pop3Cmd(t, tp, "+OK", "STAT")
}

func TestPOP3STLSHandshakeFailureClosesConnection(t *testing.T) {
	_, listener := newTestServerWithMessagesAndTLS(t, []mockMessage{
		{uidl: "msg001", size: 42, content: "From: a@example.com\r\n\r\nHello\r\n"},
	}, testPOP3TLSConfig(t))
	defer listener.Close()

	tp := pop3ConnWithDeadline(t, listener.Addr().String(), 2*time.Second)
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "STLS")
	if err := tp.PrintfLine("NOOP"); err != nil {
		t.Fatalf("send invalid TLS payload: %v", err)
	}

	line, err := tp.ReadLine()
	if err == nil && !strings.HasPrefix(line, "-ERR") {
		t.Fatalf("expected STLS failure response or connection close, got line: %s", line)
	}
	if err == nil {
		if line, err := tp.ReadLine(); err == nil {
			t.Fatalf("expected connection close after STLS failure, got line: %s", line)
		}
	}
}

func TestPOP3ImplicitTLSDoesNotAdvertiseSTLS(t *testing.T) {
	store := &mockStore{
		mailbox: &mockMailbox{
			messages: []mockMessage{{uidl: "msg001", size: 42, content: "Hello\r\n"}},
			deleted:  make(map[int]bool),
		},
	}
	tlsConfig := testPOP3TLSConfig(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	server := &Server{
		Store:       store,
		TLSConfig:   tlsConfig,
		Greeting:    "test",
		IdleTimeout: 5 * time.Second,
	}
	go func() { _ = server.Serve(tls.NewListener(listener, tlsConfig)) }()

	conn, err := tls.Dial("tcp", listener.Addr().String(), &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatalf("tls dial: %v", err)
	}
	tp := textproto.NewConn(conn)
	defer tp.Close()
	line, err := tp.ReadLine()
	if err != nil {
		t.Fatalf("greeting: %v", err)
	}
	if !strings.HasPrefix(line, "+OK") {
		t.Fatalf("unexpected greeting: %s", line)
	}
	capa := pop3Capa(t, tp)
	if capa["STLS"] {
		t.Fatalf("implicit TLS CAPA advertised STLS: %#v", capa)
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

func TestPOP3ListHidesDeletedMessages(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "+OK", "DELE 1")
	pop3Cmd(t, tp, "-ERR", "LIST 1")
	pop3Cmd(t, tp, "+OK", "LIST")

	reader := tp.DotReader()
	scanner := bufio.NewScanner(reader)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) != 1 {
		t.Fatalf("expected 1 LIST line after delete, got %d", len(lines))
	}
	if strings.HasPrefix(lines[0], "1 ") {
		t.Fatalf("deleted message LIST should be hidden, got: %s", lines[0])
	}
	if !strings.HasPrefix(lines[0], "2 ") {
		t.Fatalf("expected remaining message 2 LIST entry, got: %s", lines[0])
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

func TestPOP3RetrTopHideDeletedMessages(t *testing.T) {
	_, listener := newTestServer(t)
	defer listener.Close()

	tp := pop3Conn(t, listener.Addr().String())
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "+OK", "DELE 1")
	pop3Cmd(t, tp, "-ERR", "RETR 1")
	pop3Cmd(t, tp, "-ERR", "TOP 1 0")
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

func TestPOP3CommitDeletesErrorRestoresWireVisibility(t *testing.T) {
	mb := &commitMailbox{
		mockMailbox: &mockMailbox{
			messages: []mockMessage{
				{uidl: "msg001", size: 42, content: "From: a@example.com\r\n\r\nHello\r\n"},
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
	pop3Cmd(t, tp, "-ERR", "QUIT")

	if line := pop3Cmd(t, tp, "+OK", "LIST 1"); !strings.Contains(line, "1 42") {
		t.Fatalf("expected LIST 1 after failed QUIT to restore message size, got: %s", line)
	}
	if line := pop3Cmd(t, tp, "+OK", "UIDL 1"); !strings.Contains(line, "msg001") {
		t.Fatalf("expected UIDL 1 after failed QUIT to restore message UIDL, got: %s", line)
	}

	pop3Cmd(t, tp, "+OK", "RETR 1")
	data, err := io.ReadAll(tp.DotReader())
	if err != nil {
		t.Fatalf("read RETR body after failed QUIT: %v", err)
	}
	if !strings.Contains(string(data), "Hello") {
		t.Fatalf("expected RETR 1 body after failed QUIT, got %q", string(data))
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

func TestPOP3QuitSuccessClosesConnection(t *testing.T) {
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

	tp := pop3ConnWithDeadline(t, listener.Addr().String(), 2*time.Second)
	defer tp.Close()

	pop3Cmd(t, tp, "+OK", "USER alice")
	pop3Cmd(t, tp, "+OK", "PASS secret")
	pop3Cmd(t, tp, "+OK", "DELE 1")
	pop3Cmd(t, tp, "+OK", "QUIT")

	if line, err := tp.ReadLine(); err == nil {
		t.Fatalf("expected connection close after successful QUIT, got line: %s", line)
	}
}
