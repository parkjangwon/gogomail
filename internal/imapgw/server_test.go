package imapgw

import (
	"bufio"
	"context"
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
	if line != "* CAPABILITY IMAP4rev1 IDLE ID NAMESPACE UNSELECT SASL-IR AUTH=PLAIN\r\n" {
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

func TestServerHandlesStartTLS(t *testing.T) {
	t.Parallel()

	serverTLS := testIMAPTLSConfig(t)
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, TLSConfig: serverTLS})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "* OK gogomail IMAP4rev1 service ready\r\n" {
		t.Fatalf("greeting = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a1 CAPABILITY\r\n")); err != nil {
		t.Fatalf("write capability: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 IDLE ID NAMESPACE UNSELECT STARTTLS LOGINDISABLED\r\n" {
		t.Fatalf("pre-tls capability = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK CAPABILITY completed\r\n" {
		t.Fatalf("pre-tls capability completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write pre-tls login: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 NO [PRIVACYREQUIRED] TLS is required for LOGIN\r\n" {
		t.Fatalf("pre-tls login line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a3 STARTTLS\r\n")); err != nil {
		t.Fatalf("write starttls: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 OK [CAPABILITY IMAP4rev1 IDLE ID NAMESPACE UNSELECT SASL-IR AUTH=PLAIN] Begin TLS negotiation now\r\n" {
		t.Fatalf("starttls line = %q err = %v", line, err)
	}
	tlsClient := tls.Client(client, &tls.Config{InsecureSkipVerify: true})
	if err := tlsClient.Handshake(); err != nil {
		t.Fatalf("client handshake: %v", err)
	}
	reader = bufio.NewReader(tlsClient)
	if _, err := tlsClient.Write([]byte("a4 CAPABILITY\r\n")); err != nil {
		t.Fatalf("write tls capability: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 IDLE ID NAMESPACE UNSELECT SASL-IR AUTH=PLAIN\r\n" {
		t.Fatalf("post-tls capability = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a4 OK CAPABILITY completed\r\n" {
		t.Fatalf("post-tls capability completion = %q err = %v", line, err)
	}
	if _, err := tlsClient.Write([]byte("a5 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
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
	if line != "* CAPABILITY IMAP4rev1 IDLE ID NAMESPACE UNSELECT\r\n" {
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

func TestServerHandlesNamespaceAfterLogin(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 NAMESPACE\r\na2 LOGIN user@example.com secret\r\na3 NAMESPACE\r\n")); err != nil {
		t.Fatalf("write namespace flow: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 NO authentication required\r\n" {
		t.Fatalf("unauthenticated namespace = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* NAMESPACE ((\"\" \"/\")) NIL NIL\r\n",
		"a3 OK NAMESPACE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read namespace response: %v", err)
		}
		if line != expected {
			t.Fatalf("namespace response = %q, want %q", line, expected)
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

func TestServerHandlesIDCommand(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 ID NIL\r\n")); err != nil {
		t.Fatalf("write id: %v", err)
	}
	want := []string{
		"* ID (\"name\" \"gogomail\")\r\n",
		"a1 OK ID completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read id response: %v", err)
		}
		if line != expected {
			t.Fatalf("id response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a2 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func testIMAPTLSConfig(t *testing.T) *tls.Config {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
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
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("load key pair: %v", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
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
	if line, err = reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 IDLE ID NAMESPACE UNSELECT\r\n" {
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
		"* 0 RECENT\r\n",
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
		"* 0 RECENT\r\n",
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

func TestServerHandlesAuthenticatePlainInitialResponse(t *testing.T) {
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
	response := base64.StdEncoding.EncodeToString([]byte("\x00user@example.com\x00secret"))
	if _, err := client.Write([]byte("a1 AUTHENTICATE PLAIN " + response + "\r\n")); err != nil {
		t.Fatalf("write authenticate initial response: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK AUTHENTICATE completed\r\n" {
		t.Fatalf("authenticate initial response completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 CAPABILITY\r\n")); err != nil {
		t.Fatalf("write capability: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 IDLE ID NAMESPACE UNSELECT\r\n" {
		t.Fatalf("authenticated capability = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 OK CAPABILITY completed\r\n" {
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
	for i := 0; i < 7; i++ {
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

func TestServerHandlesUnselectAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UNSELECT\r\na4 FETCH 1 (FLAGS)\r\n")); err != nil {
		t.Fatalf("write unselect/fetch: %v", err)
	}
	want := []string{
		"a3 OK UNSELECT completed\r\n",
		"a4 NO mailbox must be selected\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read unselect response: %v", err)
		}
		if line != expected {
			t.Fatalf("unselect response = %q, want %q", line, expected)
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

func TestServerRejectsUnsupportedExpunge(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 EXPUNGE\r\na4 UID EXPUNGE 7\r\n")); err != nil {
		t.Fatalf("write expunge: %v", err)
	}
	want := []string{
		"a3 NO EXPUNGE is not supported\r\n",
		"a4 NO UID EXPUNGE is not supported\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read expunge response: %v", err)
		}
		if line != expected {
			t.Fatalf("expunge response = %q, want %q", line, expected)
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

func TestServerRejectsUnsupportedCopyMoveAndAppend(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 COPY 1 Archive\r\na4 UID COPY 7 Archive\r\na5 MOVE 1 Archive\r\na6 UID MOVE 7 Archive\r\na7 APPEND inbox NIL\r\n")); err != nil {
		t.Fatalf("write unsupported mutation commands: %v", err)
	}
	want := []string{
		"a3 NO COPY is not supported\r\n",
		"a4 NO UID COPY is not supported\r\n",
		"a5 NO MOVE is not supported\r\n",
		"a6 NO UID MOVE is not supported\r\n",
		"a7 NO APPEND is not supported\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read unsupported mutation response: %v", err)
		}
		if line != expected {
			t.Fatalf("unsupported mutation response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a8 LOGOUT\r\n")); err != nil {
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
	for i := 0; i < 7; i++ {
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
	for i := 0; i < 7; i++ {
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
	if err := client.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* 4 EXISTS\r\n" {
		t.Fatalf("live idle event = %q err = %v", line, err)
	}
	if err := client.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear read deadline: %v", err)
	}
	if _, err := client.Write([]byte("DONE\r\n")); err != nil {
		t.Fatalf("write done: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 OK IDLE completed\r\n" {
		t.Fatalf("idle completion = %q err = %v", line, err)
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

func TestServerHandlesLsubAfterLogin(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LSUB \"\" \"INBOX\"\r\n")); err != nil {
		t.Fatalf("write login/lsub: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LSUB (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"a2 OK LSUB completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read lsub response: %v", err)
		}
		if line != expected {
			t.Fatalf("lsub response = %q, want %q", line, expected)
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

func TestServerListsHierarchyRoot(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" \"\"\r\na3 LSUB \"\" \"\"\r\n")); err != nil {
		t.Fatalf("write login/list root: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\Noselect) \"/\" \"\"\r\n",
		"a2 OK LIST completed\r\n",
		"* LSUB (\\Noselect) \"/\" \"\"\r\n",
		"a3 OK LSUB completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read list root response: %v", err)
		}
		if line != expected {
			t.Fatalf("list root response = %q, want %q", line, expected)
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

func TestServerHandlesSubscriptionCommandsAfterLogin(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SUBSCRIBE inbox\r\na3 UNSUBSCRIBE inbox\r\n")); err != nil {
		t.Fatalf("write subscription commands: %v", err)
	}
	want := []string{
		"a1 OK LOGIN completed\r\n",
		"a2 OK SUBSCRIBE completed\r\n",
		"a3 OK UNSUBSCRIBE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read subscription response: %v", err)
		}
		if line != expected {
			t.Fatalf("subscription response = %q, want %q", line, expected)
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

func TestServerRejectsUnsupportedMailboxMutations(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 CREATE Projects\r\na2 LOGIN user@example.com secret\r\na3 CREATE Projects\r\na4 DELETE Projects\r\na5 RENAME Projects Archive/Projects\r\n")); err != nil {
		t.Fatalf("write mailbox mutations: %v", err)
	}
	want := []string{
		"a1 NO authentication required\r\n",
		"a2 OK LOGIN completed\r\n",
		"a3 NO CREATE is not supported\r\n",
		"a4 NO DELETE is not supported\r\n",
		"a5 NO RENAME is not supported\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read mailbox mutation response: %v", err)
		}
		if line != expected {
			t.Fatalf("mailbox mutation response = %q, want %q", line, expected)
		}
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
	for i := 0; i < 7; i++ {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7:8 (FLAGS RFC822.SIZE)\r\n")); err != nil {
		t.Fatalf("write uid fetch set: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41)\r\n",
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 1:* (FLAGS RFC822.SIZE)\r\n")); err != nil {
		t.Fatalf("write fetch: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41)\r\n",
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

func TestServerHandlesSearchAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH ALL\r\na4 UID SEARCH ALL\r\na5 SEARCH UID 8:9\r\na6 SEARCH UNSEEN SINCE 04-May-2026 LARGER 20\r\na7 UID SEARCH ALL FROM archive SENTBEFORE 04-May-2026\r\na8 SEARCH NOT SEEN\r\na9 UID SEARCH OR FROM sender BCC hidden\r\na10 SEARCH CHARSET UTF-8 SUBJECT IMAP\r\na11 UID SEARCH CHARSET US-ASCII ALL\r\na12 SEARCH CHARSET ISO-8859-1 ALL\r\na13 SEARCH 2:*\r\na14 UID SEARCH 1:* SUBJECT Archive\r\n")); err != nil {
		t.Fatalf("write search: %v", err)
	}
	want := []string{
		"* SEARCH 1 2\r\n",
		"a3 OK SEARCH completed\r\n",
		"* SEARCH 7 8\r\n",
		"a4 OK UID SEARCH completed\r\n",
		"* SEARCH 2\r\n",
		"a5 OK SEARCH completed\r\n",
		"* SEARCH 2\r\n",
		"a6 OK SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a7 OK UID SEARCH completed\r\n",
		"* SEARCH 2\r\n",
		"a8 OK SEARCH completed\r\n",
		"* SEARCH 7 8\r\n",
		"a9 OK UID SEARCH completed\r\n",
		"* SEARCH 1\r\n",
		"a10 OK SEARCH completed\r\n",
		"* SEARCH 7 8\r\n",
		"a11 OK UID SEARCH completed\r\n",
		"a12 NO [BADCHARSET (US-ASCII UTF-8)] SEARCH charset is unsupported\r\n",
		"* SEARCH 2\r\n",
		"a13 OK SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a14 OK UID SEARCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read search response: %v", err)
		}
		if line != expected {
			t.Fatalf("search response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a10 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesFlagSearchAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH UNSEEN\r\na4 UID SEARCH FLAGGED\r\na5 SEARCH DRAFT\r\na6 UID SEARCH UNDRAFT\r\na7 SEARCH DELETED\r\na8 UID SEARCH UNDELETED\r\na9 SEARCH RECENT\r\na10 UID SEARCH OLD\r\na11 SEARCH NEW\r\na12 SEARCH KEYWORD custom\r\na13 UID SEARCH UNKEYWORD custom\r\na14 SEARCH KEYWORD bad%flag\r\n")); err != nil {
		t.Fatalf("write flag search: %v", err)
	}
	want := []string{
		"* SEARCH 2\r\n",
		"a3 OK SEARCH completed\r\n",
		"* SEARCH 7\r\n",
		"a4 OK UID SEARCH completed\r\n",
		"* SEARCH 2\r\n",
		"a5 OK SEARCH completed\r\n",
		"* SEARCH 7\r\n",
		"a6 OK UID SEARCH completed\r\n",
		"* SEARCH\r\n",
		"a7 OK SEARCH completed\r\n",
		"* SEARCH 7 8\r\n",
		"a8 OK UID SEARCH completed\r\n",
		"* SEARCH\r\n",
		"a9 OK SEARCH completed\r\n",
		"* SEARCH 7 8\r\n",
		"a10 OK UID SEARCH completed\r\n",
		"* SEARCH\r\n",
		"a11 OK SEARCH completed\r\n",
		"* SEARCH\r\n",
		"a12 OK SEARCH completed\r\n",
		"* SEARCH 7 8\r\n",
		"a13 OK UID SEARCH completed\r\n",
		"a14 BAD SEARCH criteria are unsupported\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read flag search response: %v", err)
		}
		if line != expected {
			t.Fatalf("flag search response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a15 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesDateSearchAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH SINCE 05-May-2026\r\na4 UID SEARCH BEFORE 05-May-2026\r\na5 SEARCH ON 05-May-2026\r\na6 UID SEARCH SENTON 03-May-2026\r\na7 SEARCH SENTSINCE 04-May-2026\r\na8 UID SEARCH SENTBEFORE 04-May-2026\r\n")); err != nil {
		t.Fatalf("write date search: %v", err)
	}
	want := []string{
		"* SEARCH 1\r\n",
		"a3 OK SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a4 OK UID SEARCH completed\r\n",
		"* SEARCH 1\r\n",
		"a5 OK SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a6 OK UID SEARCH completed\r\n",
		"* SEARCH 1\r\n",
		"a7 OK SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a8 OK UID SEARCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read date search response: %v", err)
		}
		if line != expected {
			t.Fatalf("date search response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a9 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesSizeSearchAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH LARGER 20\r\na4 UID SEARCH SMALLER 20\r\n")); err != nil {
		t.Fatalf("write size search: %v", err)
	}
	want := []string{
		"* SEARCH 2\r\n",
		"a3 OK SEARCH completed\r\n",
		"* SEARCH 7\r\n",
		"a4 OK UID SEARCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read size search response: %v", err)
		}
		if line != expected {
			t.Fatalf("size search response = %q, want %q", line, expected)
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

func TestServerHandlesTextSearchAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH SUBJECT IMAP\r\na4 UID SEARCH FROM archive\r\na5 SEARCH TO target\r\na6 UID SEARCH CC review\r\na7 SEARCH BCC hidden\r\n")); err != nil {
		t.Fatalf("write text search: %v", err)
	}
	want := []string{
		"* SEARCH 1\r\n",
		"a3 OK SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a4 OK UID SEARCH completed\r\n",
		"* SEARCH 1\r\n",
		"a5 OK SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a6 OK UID SEARCH completed\r\n",
		"* SEARCH 2\r\n",
		"a7 OK SEARCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read text search response: %v", err)
		}
		if line != expected {
			t.Fatalf("text search response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a8 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesBodySearchAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH BODY archived\r\na4 UID SEARCH TEXT Archive\r\na5 SEARCH BODY Subject\r\na6 UID SEARCH HEADER Subject Archive\r\n")); err != nil {
		t.Fatalf("write body search: %v", err)
	}
	want := []string{
		"* SEARCH 2\r\n",
		"a3 OK SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a4 OK UID SEARCH completed\r\n",
		"* SEARCH\r\n",
		"a5 OK SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a6 OK UID SEARCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read body search response: %v", err)
		}
		if line != expected {
			t.Fatalf("body search response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a7 LOGOUT\r\n")); err != nil {
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
	for i := 0; i < 7; i++ {
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
	for i := 0; i < 7; i++ {
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

func TestServerHandlesFetchMacros(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 1 FAST\r\na4 FETCH 1 FULL\r\n")); err != nil {
		t.Fatalf("write fetch macros: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 INTERNALDATE \"05-May-2026 12:34:56 +0900\")\r\n",
		"a3 OK FETCH completed\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 INTERNALDATE \"05-May-2026 12:34:56 +0900\" ENVELOPE (\"Tue, 05 May 2026 12:34:56 +0900\" \"Hello IMAP\" ((\"Sender\" NIL \"sender\" \"example.net\")) ((\"Sender\" NIL \"sender\" \"example.net\")) ((\"Sender\" NIL \"sender\" \"example.net\")) ((\"User\" NIL \"user\" \"example.com\")) NIL NIL NIL \"<message-7@example.net>\") BODY (\"TEXT\" \"PLAIN\" (\"CHARSET\" \"UTF-8\") NIL NIL \"7BIT\" 11 1))\r\n",
		"a4 OK FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read fetch macro response: %v", err)
		}
		if line != expected {
			t.Fatalf("fetch macro response = %q, want %q", line, expected)
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
	for i := 0; i < 7; i++ {
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

func TestServerHandlesUIDFetchPartialBodyAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7 BODY.PEEK[]<6.5>\r\n")); err != nil {
		t.Fatalf("write uid fetch partial body: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read partial literal header: %v", err)
	}
	if line != "* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 BODY[]<6> {5}\r\n" {
		t.Fatalf("partial literal header = %q", line)
	}
	body := make([]byte, 5)
	if _, err := io.ReadFull(reader, body); err != nil {
		t.Fatalf("read partial literal: %v", err)
	}
	if string(body) != "world" {
		t.Fatalf("partial body = %q", body)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("partial close = %q err = %v", line, err)
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
	for i := 0; i < 7; i++ {
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

func TestServerHandlesFetchHeaderAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 2 RFC822.HEADER\r\n")); err != nil {
		t.Fatalf("write fetch header: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read fetch header literal header: %v", err)
	}
	if line != "* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41 BODY[HEADER] {20}\r\n" {
		t.Fatalf("fetch header literal header = %q", line)
	}
	header := make([]byte, 20)
	if _, err := io.ReadFull(reader, header); err != nil {
		t.Fatalf("read fetch header literal: %v", err)
	}
	if string(header) != "Subject: Archive\r\n\r\n" {
		t.Fatalf("fetch header = %q", header)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("fetch header close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a3 OK FETCH completed\r\n" {
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

func TestServerHandlesUIDFetchHeaderFieldsAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 9 BODY.PEEK[HEADER.FIELDS (Subject)]\r\n")); err != nil {
		t.Fatalf("write uid fetch header fields: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read header fields literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER] {18}\r\n" {
		t.Fatalf("header fields literal header = %q", line)
	}
	header := make([]byte, 18)
	if _, err := io.ReadFull(reader, header); err != nil {
		t.Fatalf("read header fields literal: %v", err)
	}
	if string(header) != "Subject: Hello\r\n\r\n" {
		t.Fatalf("header fields = %q", header)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("header fields close = %q err = %v", line, err)
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

func TestServerHandlesUIDFetchHeaderFieldsNotAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 9 BODY.PEEK[HEADER.FIELDS.NOT (From)]\r\n")); err != nil {
		t.Fatalf("write uid fetch header fields not: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read header fields not literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER] {18}\r\n" {
		t.Fatalf("header fields not literal header = %q", line)
	}
	header := make([]byte, 18)
	if _, err := io.ReadFull(reader, header); err != nil {
		t.Fatalf("read header fields not literal: %v", err)
	}
	if string(header) != "Subject: Hello\r\n\r\n" {
		t.Fatalf("header fields not = %q", header)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("header fields not close = %q err = %v", line, err)
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
	for i := 0; i < 7; i++ {
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

func TestServerHandlesUIDFetchSinglePartTextAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 9 BODY.PEEK[1]\r\n")); err != nil {
		t.Fatalf("write uid fetch part text: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read part text literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[1] {17}\r\n" {
		t.Fatalf("part text literal header = %q", line)
	}
	text := make([]byte, 17)
	if _, err := io.ReadFull(reader, text); err != nil {
		t.Fatalf("read part text literal: %v", err)
	}
	if string(text) != "hello header body" {
		t.Fatalf("part text = %q", text)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("part text close = %q err = %v", line, err)
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

func TestServerHandlesUIDFetchSinglePartMIMEAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 9 BODY.PEEK[1.MIME]\r\n")); err != nil {
		t.Fatalf("write uid fetch part mime: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read part mime literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[1.MIME] {2}\r\n" {
		t.Fatalf("part mime literal header = %q", line)
	}
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		t.Fatalf("read part mime literal: %v", err)
	}
	if string(header) != "\r\n" {
		t.Fatalf("part mime = %q", header)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("part mime close = %q err = %v", line, err)
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
	for i := 0; i < 7; i++ {
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

func TestServerHandlesStoreAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 STORE 1:2 +FLAGS (\\Seen \\Flagged)\r\n")); err != nil {
		t.Fatalf("write store: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged))\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged))\r\n",
		"a3 OK STORE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read store response: %v", err)
		}
		if line != expected {
			t.Fatalf("store response = %q, want %q", line, expected)
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

func TestServerHandlesStoreSilentAfterSelect(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 STORE 1 +FLAGS.SILENT (\\Seen \\Flagged)\r\n")); err != nil {
		t.Fatalf("write store silent: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 OK STORE completed\r\n" {
		t.Fatalf("store silent completion = %q err = %v", line, err)
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
	for i := 0; i < 7; i++ {
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

func TestParseIMAPPartialBody(t *testing.T) {
	t.Parallel()

	got, ok := imapFetchPartialBody([]string{"BODY.PEEK[]<12.34>"})
	if !ok {
		t.Fatal("imapFetchPartialBody rejected valid partial")
	}
	if got.offset != 12 || got.count != 34 {
		t.Fatalf("partial = %+v, want offset 12 count 34", got)
	}
	if _, ok := imapFetchPartialBody([]string{"BODY[]"}); ok {
		t.Fatal("imapFetchPartialBody accepted full body fetch")
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

func TestFilterIMAPHeaderFields(t *testing.T) {
	t.Parallel()

	got := filterIMAPHeaderFields([]byte("Subject: Hi\r\n folded\r\nFrom: sender@test\r\nTo: user@test\r\n\r\n"), []string{"subject", "to"}, false)
	want := "Subject: Hi\r\n folded\r\nTo: user@test\r\n\r\n"
	if string(got) != want {
		t.Fatalf("filtered header = %q, want %q", got, want)
	}
	got = filterIMAPHeaderFields([]byte("Subject: Hi\r\nFrom: sender@test\r\nTo: user@test\r\n\r\n"), []string{"from"}, true)
	want = "Subject: Hi\r\nTo: user@test\r\n\r\n"
	if string(got) != want {
		t.Fatalf("excluded header = %q, want %q", got, want)
	}
}

func TestIMAPBodyStructureDefersMultipartHeaders(t *testing.T) {
	t.Parallel()

	header := []byte("Content-Type: multipart/mixed; boundary=frontier\r\n\r\n")
	got := imapBodyStructureFromHeader(MessageSummary{Size: 123}, header)
	want := `("TEXT" "PLAIN" ("CHARSET" "UTF-8") NIL NIL "7BIT" 123 1 NIL NIL NIL NIL)`
	if got != want {
		t.Fatalf("bodystructure = %q, want conservative single-part fallback %q", got, want)
	}
}

func TestServerHandlesFetchBodyStructureFromMessageHeaders(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 10 BODYSTRUCTURE\r\n")); err != nil {
		t.Fatalf("write uid fetch bodystructure: %v", err)
	}
	bodySize := len("Content-Type: text/html; charset=utf-8; format=flowed\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n<p>Hello</p>")
	partSize := len("<p>Hello</p>")
	want := []string{
		fmt.Sprintf("* 4 FETCH (UID 10 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODYSTRUCTURE (\"TEXT\" \"HTML\" (\"CHARSET\" \"utf-8\" \"FORMAT\" \"flowed\") NIL NIL \"QUOTED-PRINTABLE\" %d 1 NIL NIL NIL NIL))\r\n", bodySize, partSize),
		"a3 OK UID FETCH completed\r\n",
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
}

func TestServerHandlesFetchMultipartBodyStructure(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 11 BODYSTRUCTURE\r\n")); err != nil {
		t.Fatalf("write uid fetch bodystructure: %v", err)
	}
	bodySize := len(testMultipartBody())
	want := []string{
		fmt.Sprintf("* 5 FETCH (UID 11 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODYSTRUCTURE ((\"TEXT\" \"PLAIN\" (\"CHARSET\" \"utf-8\") NIL NIL \"7BIT\" 5 1 NIL NIL NIL NIL) (\"APPLICATION\" \"PDF\" (\"NAME\" \"report.pdf\") NIL NIL \"BASE64\" 12 NIL (\"ATTACHMENT\" (\"FILENAME\" \"report.pdf\")) NIL NIL) \"MIXED\" (\"BOUNDARY\" \"frontier\") NIL NIL NIL))\r\n", bodySize),
		"a3 OK UID FETCH completed\r\n",
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
}

func TestServerHandlesCombinedBodyStructureAndHeaderFetch(t *testing.T) {
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 11 (BODYSTRUCTURE BODY.PEEK[HEADER])\r\n")); err != nil {
		t.Fatalf("write uid fetch bodystructure/header: %v", err)
	}
	bodySize := len(testMultipartBody())
	header := "Content-Type: multipart/mixed; boundary=frontier\r\n\r\n"
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read combined fetch line: %v", err)
	}
	wantPrefix := fmt.Sprintf("* 5 FETCH (UID 11 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODYSTRUCTURE ((\"TEXT\" \"PLAIN\" (\"CHARSET\" \"utf-8\") NIL NIL \"7BIT\" 5 1 NIL NIL NIL NIL) (\"APPLICATION\" \"PDF\" (\"NAME\" \"report.pdf\") NIL NIL \"BASE64\" 12 NIL (\"ATTACHMENT\" (\"FILENAME\" \"report.pdf\")) NIL NIL) \"MIXED\" (\"BOUNDARY\" \"frontier\") NIL NIL NIL) BODY[HEADER] {%d}\r\n", bodySize, len(header))
	if line != wantPrefix {
		t.Fatalf("combined fetch line = %q, want %q", line, wantPrefix)
	}
	literal := make([]byte, len(header))
	if _, err := io.ReadFull(reader, literal); err != nil {
		t.Fatalf("read combined header literal: %v", err)
	}
	if string(literal) != header {
		t.Fatalf("combined header literal = %q, want %q", literal, header)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("fetch close line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("completion line = %q err = %v", line, err)
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
		{ID: "message-1", UID: 7, SequenceNumber: 1, InternalDate: time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC), Envelope: Envelope{Subject: "Hello IMAP", From: []Address{{Name: "Sender", Mailbox: "sender", Host: "example.net"}}, To: []Address{{Name: "Target User", Mailbox: "target", Host: "example.com"}}, Date: time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)}, Flags: MessageFlags{Read: true, Starred: true}, Size: 11},
		{ID: "message-2", UID: 8, SequenceNumber: 2, InternalDate: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC), Envelope: Envelope{Subject: "Archive", From: []Address{{Name: "Archive Bot", Mailbox: "archive", Host: "example.net"}}, Cc: []Address{{Name: "Review Desk", Mailbox: "review", Host: "example.com"}}, Bcc: []Address{{Name: "Hidden Desk", Mailbox: "hidden", Host: "example.com"}}, Date: time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)}, Flags: MessageFlags{Draft: true}, Size: 42},
	}, nil
}

func (fakeBackend) FetchMessage(_ context.Context, req FetchMessageRequest) (Message, error) {
	internalDate := time.Date(2026, 5, 5, 12, 34, 56, 0, time.FixedZone("KST", 9*60*60))
	body := "hello world"
	size := int64(len(body))
	if req.UID == 8 {
		body = "Subject: Archive\r\n\r\narchived body content"
		size = int64(len(body))
	}
	if req.UID == 9 {
		body = "Subject: Hello\r\nFrom: sender@test\r\n\r\nhello header body"
		size = int64(len(body))
	}
	if req.UID == 10 {
		body = "Content-Type: text/html; charset=utf-8; format=flowed\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n<p>Hello</p>"
		size = int64(len(body))
	}
	if req.UID == 11 {
		body = testMultipartBody()
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

func testMultipartBody() string {
	return strings.Join([]string{
		"Content-Type: multipart/mixed; boundary=frontier",
		"",
		"--frontier",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: 7bit",
		"",
		"hello",
		"--frontier",
		"Content-Type: application/pdf; name=\"report.pdf\"",
		"Content-Transfer-Encoding: base64",
		"Content-Disposition: attachment; filename=\"report.pdf\"",
		"",
		"UEZGREFUQQ==",
		"--frontier--",
		"",
	}, "\r\n")
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
