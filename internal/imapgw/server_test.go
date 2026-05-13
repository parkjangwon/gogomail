package imapgw

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	messageparse "github.com/gogomail/gogomail/internal/message"
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
		{name: "negative max connections", opts: ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true, MaxConnections: -1}},
		{name: "negative read timeout", opts: ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true, ReadTimeout: -time.Second}},
		{name: "negative write timeout", opts: ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true, WriteTimeout: -time.Second}},
		{name: "negative idle timeout", opts: ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true, IdleTimeout: -time.Second}},
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

func TestServerServeConnAppliesReadTimeout(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{
		Addr:              "127.0.0.1:0",
		Backend:           fakeBackend{},
		AllowInsecureAuth: true,
		ReadTimeout:       20 * time.Millisecond,
		WriteTimeout:      time.Second,
	})
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
	if line, err := reader.ReadString('\n'); err != nil || !strings.HasPrefix(line, "* OK ") {
		t.Fatalf("greeting = %q, err = %v", line, err)
	}

	select {
	case err := <-errCh:
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			t.Fatalf("ServeConn err = %v, want timeout", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ServeConn did not return after read timeout")
	}
}

func TestServerServeConnAppliesIdleTimeout(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{
		Addr:              "127.0.0.1:0",
		Backend:           fakeBackend{},
		AllowInsecureAuth: true,
		ReadTimeout:       time.Second,
		WriteTimeout:      time.Second,
		IdleTimeout:       20 * time.Millisecond,
	})
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
	if line, err := reader.ReadString('\n'); err != nil || !strings.HasPrefix(line, "* OK ") {
		t.Fatalf("greeting = %q, err = %v", line, err)
	}
	if _, err := io.WriteString(client, "a1 LOGIN user@example.com secret\r\n"); err != nil {
		t.Fatalf("write LOGIN: %v", err)
	}
	readUntilPrefix(t, reader, "a1 OK ")
	if _, err := io.WriteString(client, "a2 SELECT INBOX\r\n"); err != nil {
		t.Fatalf("write SELECT: %v", err)
	}
	readUntilPrefix(t, reader, "a2 OK ")
	if _, err := io.WriteString(client, "a3 IDLE\r\n"); err != nil {
		t.Fatalf("write IDLE: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ idling\r\n" {
		t.Fatalf("idle continuation = %q, err = %v", line, err)
	}

	select {
	case err := <-errCh:
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			t.Fatalf("ServeConn err = %v, want timeout", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ServeConn did not return after idle timeout")
	}
}

func readUntilPrefix(t *testing.T, reader *bufio.Reader, prefix string) string {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read %q response: %v", prefix, err)
		}
		if strings.HasPrefix(line, prefix) {
			return line
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for response prefix %q; last line = %q", prefix, line)
		}
	}
}

func TestServerServeRejectsConnectionsOverLimit(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: "127.0.0.1:0", Backend: fakeBackend{}, AllowInsecureAuth: true, MaxConnections: 1})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	defer listener.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(listener)
	}()

	first, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial first connection: %v", err)
	}
	defer first.Close()
	if err := first.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set first deadline: %v", err)
	}
	firstReader := bufio.NewReader(first)
	if line, err := firstReader.ReadString('\n'); err != nil || !strings.HasPrefix(line, "* OK ") {
		t.Fatalf("first greeting = %q, err = %v", line, err)
	}
	if err := first.SetDeadline(time.Time{}); err != nil {
		t.Fatalf("clear first deadline: %v", err)
	}

	second, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial second connection: %v", err)
	}
	defer second.Close()
	if err := second.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set second deadline: %v", err)
	}
	secondReader := bufio.NewReader(second)
	line, err := secondReader.ReadString('\n')
	if err != nil {
		t.Fatalf("read over-limit response: %v", err)
	}
	if line != "* BYE [ALERT] gogomail IMAP4rev1 server connection limit reached\r\n" {
		t.Fatalf("over-limit response = %q", line)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close first connection: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		third, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Fatalf("dial third connection: %v", err)
		}
		if err := third.SetDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatalf("set third deadline: %v", err)
		}
		line, err := bufio.NewReader(third).ReadString('\n')
		_ = third.Close()
		if err != nil {
			t.Fatalf("read third response: %v", err)
		}
		if strings.HasPrefix(line, "* OK ") {
			break
		}
		if line != "* BYE [ALERT] gogomail IMAP4rev1 server connection limit reached\r\n" {
			t.Fatalf("third response = %q", line)
		}
		if time.Now().After(deadline) {
			t.Fatalf("connection slot was not released before deadline; last response = %q", line)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	if err := <-errCh; !errors.Is(err, ErrServerClosed) {
		t.Fatalf("Serve returned %v, want ErrServerClosed", err)
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

func TestReadIMAPLineRejectsOverLimitLineBeforeNewline(t *testing.T) {
	t.Parallel()

	reader := bufio.NewReaderSize(strings.NewReader(strings.Repeat("A", maxIMAPCommandLineBytes+1)), 32)

	if _, err := readIMAPLine(reader, maxIMAPCommandLineBytes); err == nil || !strings.Contains(err.Error(), "command line is too long") {
		t.Fatalf("readIMAPLine err = %v, want over-limit rejection", err)
	}
}

func TestServerReportsOversizedCommandLiteralBeforeClosing(t *testing.T) {
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
	command := fmt.Sprintf("a1 APPEND INBOX {%d}\r\n", maxIMAPCommandLiteralBytes+1)
	if _, err := client.Write([]byte(command)); err != nil {
		t.Fatalf("write oversized literal command: %v", err)
	}
	want := []string{
		"a1 BAD command literal is too large\r\n",
		"* BYE gogomail IMAP4rev1 server closing connection after framing error\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read oversized literal response: %v", err)
		}
		if line != expected {
			t.Fatalf("oversized literal response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestReadCommandLineRejectsCumulativeOversizedLiterals(t *testing.T) {
	t.Parallel()

	firstLiteralSize := maxIMAPCommandLiteralBytes - 1
	input := fmt.Sprintf("a1 LOGIN {%d}\r\n%s {2}\r\n", firstLiteralSize, strings.Repeat("a", firstLiteralSize))
	reader := bufio.NewReader(strings.NewReader(input))
	writer := bufio.NewWriter(&bytes.Buffer{})
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	_, _, err = server.readCommandLine(reader, writer, &imapConnState{})
	if err == nil || !strings.Contains(err.Error(), "command literal is too large") {
		t.Fatalf("readCommandLine err = %v, want cumulative literal rejection", err)
	}
}

func TestServerRejectsLFOnlyCommandLineBeforeClosing(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 NOOP\n")); err != nil {
		t.Fatalf("write lf-only command: %v", err)
	}
	want := []string{
		"a1 BAD command line must end with CRLF\r\n",
		"* BYE gogomail IMAP4rev1 server closing connection after framing error\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read lf-only command response: %v", err)
		}
		if line != expected {
			t.Fatalf("lf-only command response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestReadCommandLineRejectsLFOnlyLiteralSuffix(t *testing.T) {
	t.Parallel()

	input := "a1 LOGIN {4}\r\nuser {6}\nsecret\r\n"
	reader := bufio.NewReader(strings.NewReader(input))
	writer := bufio.NewWriter(&bytes.Buffer{})
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	_, _, err = server.readCommandLine(reader, writer, &imapConnState{})
	if err == nil || !strings.Contains(err.Error(), "command line must end with CRLF") {
		t.Fatalf("readCommandLine err = %v, want LF-only literal suffix rejection", err)
	}
}

func TestServerRejectsLeadingZeroCommandLiteralSize(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 APPEND INBOX {001}\r\n")); err != nil {
		t.Fatalf("write leading-zero literal command: %v", err)
	}
	want := []string{
		"a1 BAD command literal size is invalid\r\n",
		"* BYE gogomail IMAP4rev1 server closing connection after framing error\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read leading-zero literal response: %v", err)
		}
		if line != expected {
			t.Fatalf("leading-zero literal response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsSignedCommandLiteralSize(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 APPEND INBOX {+1}\r\n")); err != nil {
		t.Fatalf("write signed literal command: %v", err)
	}
	want := []string{
		"a1 BAD command literal size is invalid\r\n",
		"* BYE gogomail IMAP4rev1 server closing connection after framing error\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read signed literal response: %v", err)
		}
		if line != expected {
			t.Fatalf("signed literal response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
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
	if line != "* OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT SASL-IR AUTH=PLAIN] gogomail IMAP4rev1 service ready\r\n" {
		t.Fatalf("greeting = %q", line)
	}

	if _, err := client.Write([]byte("a1 CAPABILITY\r\n")); err != nil {
		t.Fatalf("write capability: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read capability untagged: %v", err)
	}
	if line != "* CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT SASL-IR AUTH=PLAIN\r\n" {
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

func TestServerRejectsArgumentsForAnyStateNoArgCommands(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 CAPABILITY extra\r\na2 NOOP extra\r\na3 LOGOUT extra\r\na4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write no-arg any-state commands: %v", err)
	}
	want := []string{
		"a1 BAD CAPABILITY does not accept arguments\r\n",
		"a2 BAD NOOP does not accept arguments\r\n",
		"a3 BAD LOGOUT does not accept arguments\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a4 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read no-arg any-state response: %v", err)
		}
		if line != expected {
			t.Fatalf("no-arg any-state response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsMalformedCommandTags(t *testing.T) {
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
	if _, err := client.Write([]byte("* NOOP\r\na]1 NOOP\r\na*1 NOOP\r\na+1 NOOP\r\na2 LOGOUT\r\n")); err != nil {
		t.Fatalf("write malformed tags: %v", err)
	}
	want := []string{
		"* BAD malformed command\r\n",
		"* BAD malformed command\r\n",
		"* BAD malformed command\r\n",
		"* BAD malformed command\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a2 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read malformed tag response: %v", err)
		}
		if line != expected {
			t.Fatalf("malformed tag response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsControlCharactersInAtoms(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 NO\x00OP\r\na2 LOGOUT\r\n")); err != nil {
		t.Fatalf("write atom control: %v", err)
	}
	want := []string{
		"a1 BAD malformed command\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a2 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read atom control response: %v", err)
		}
		if line != expected {
			t.Fatalf("atom control response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsMalformedLiteralPlacement(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com {6+}oops\r\na2 APPEND inbox {3+}\r\nabc)BAD\r\na3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write malformed literal placement: %v", err)
	}
	want := []string{
		"a1 BAD malformed command\r\n",
		"a2 BAD malformed command\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a3 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read malformed literal placement response: %v", err)
		}
		if line != expected {
			t.Fatalf("malformed literal placement response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsMalformedQuotedCommandArguments(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN \"user\"secret pass\r\na2 LOGIN \"user\\n\" pass\r\na3 LOGIN \"user\x80\" pass\r\na4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write malformed quoted arguments: %v", err)
	}
	want := []string{
		"a1 BAD malformed command\r\n",
		"a2 BAD malformed command\r\n",
		"a3 BAD malformed command\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a4 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read malformed quoted argument response: %v", err)
		}
		if line != expected {
			t.Fatalf("malformed quoted argument response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsMalformedCommandAtoms(t *testing.T) {
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
	if _, err := client.Write([]byte("a0\x80 NOOP\r\na1 CAPABILITY)\r\na2 LOGIN user@example.com secret\r\na3 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write malformed command atom setup: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* BAD malformed command\r\n" {
		t.Fatalf("non-ascii tag response = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 BAD malformed command\r\n" {
		t.Fatalf("malformed command response = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a4 UID FETCH] 7 (FLAGS)\r\na5 N\x80OOP\r\na6 UID F\x80ETCH 7 (FLAGS)\r\na7 LOGOUT\r\n")); err != nil {
		t.Fatalf("write malformed uid subcommand: %v", err)
	}
	want := []string{
		"a4 BAD malformed command\r\n",
		"a5 BAD malformed command\r\n",
		"a6 BAD malformed command\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a7 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read malformed command atom response: %v", err)
		}
		if line != expected {
			t.Fatalf("malformed command atom response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerValidatesUIDSubcommandBeforeSelectedState(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 UID\r\na3 UID FETCH]\r\na4 UID BOGUS\r\na5 UID FETCH\r\na6 UID STORE\r\na7 UID EXPUNGE\r\na8 UID COPY 7 &Jjo!\r\na9 UID MOVE 7 &Jjo!\r\na10 UID FETCH 7 (FLAGS)\r\na11 LOGOUT\r\n")); err != nil {
		t.Fatalf("write uid commands: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a2 BAD UID requires subcommand\r\n",
		"a3 BAD malformed command\r\n",
		"a4 BAD UID command not implemented\r\n",
		"a5 BAD UID FETCH requires UID set and data items\r\n",
		"a6 BAD UID STORE requires UID, mode, and flags\r\n",
		"a7 BAD UID EXPUNGE requires UID set\r\n",
		"a8 BAD UID COPY destination mailbox name is not valid modified UTF-7\r\n",
		"a9 BAD UID MOVE destination mailbox name is not valid modified UTF-7\r\n",
		"a10 NO mailbox must be selected\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a11 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read uid response: %v", err)
		}
		if line != expected {
			t.Fatalf("uid response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerValidatesUIDSubcommandBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 UID\r\na2 UID FETCH]\r\na3 UID BOGUS\r\na4 UID FETCH\r\na5 UID STORE\r\na6 UID STORE 7 +FLAGS \\Seen)\r\na7 UID FETCH +7 (FLAGS)\r\na8 UID STORE +7 +FLAGS (\\Seen)\r\na9 UID EXPUNGE +7\r\na10 UID COPY +7 Archive\r\na11 UID MOVE +7 Archive\r\na12 UID EXPUNGE\r\na13 UID COPY 7 &Jjo!\r\na14 UID MOVE 7 &Jjo!\r\na15 UID FETCH \"7: 8\" (FLAGS)\r\na16 UID FETCH 7 (FLAGS)\r\na17 UID FETCH 7 ((FLAGS))\r\na18 UID SEARCH\r\na19 UID SEARCH RETURN (COUNT)\r\na20 UID SEARCH CHARSET UTF-8\r\na21 UID SORT (DATE) UTF-8\r\na22 UID SORT DATE UTF-8 ALL\r\na23 UID THREAD ORDEREDSUBJECT UTF-8\r\na24 UID THREAD REFERENCES UTF-8 ALL\r\na25 UID THREAD ORDEREDSUBJECT UTF-8 ALL\r\na26 UID SEARCH ALL\r\na27 UID ESEARCH RETURN (COUNT) ALL\r\na28 LOGOUT\r\n")); err != nil {
		t.Fatalf("write uid auth commands: %v", err)
	}
	want := []string{
		"a1 BAD UID requires subcommand\r\n",
		"a2 BAD malformed command\r\n",
		"a3 BAD UID command not implemented\r\n",
		"a4 BAD UID FETCH requires UID set and data items\r\n",
		"a5 BAD UID STORE requires UID, mode, and flags\r\n",
		"a6 BAD UID STORE flags are unsupported\r\n",
		"a7 BAD UID FETCH requires a positive UID set\r\n",
		"a8 BAD UID STORE requires a positive UID set\r\n",
		"a9 BAD UID EXPUNGE requires a positive UID set\r\n",
		"a10 BAD UID COPY requires a positive UID set\r\n",
		"a11 BAD UID MOVE requires a positive UID set\r\n",
		"a12 BAD UID EXPUNGE requires UID set\r\n",
		"a13 BAD UID COPY destination mailbox name is not valid modified UTF-7\r\n",
		"a14 BAD UID MOVE destination mailbox name is not valid modified UTF-7\r\n",
		"a15 BAD UID FETCH requires a positive UID set\r\n",
		"a16 NO authentication required\r\n",
		"a17 BAD FETCH data item list is invalid\r\n",
		"a18 BAD SEARCH requires criteria\r\n",
		"a19 BAD SEARCH requires criteria\r\n",
		"a20 BAD SEARCH requires criteria\r\n",
		"a21 BAD SORT requires sort criteria, charset, and search criteria\r\n",
		"a22 BAD SORT arguments are unsupported\r\n",
		"a23 BAD THREAD requires algorithm, charset, and search criteria\r\n",
		"a24 BAD THREAD algorithm is unsupported\r\n",
		"a25 NO authentication required\r\n",
		"a26 NO authentication required\r\n",
		"a27 BAD ESEARCH command requires MULTISEARCH capability\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a28 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read uid auth response: %v", err)
		}
		if line != expected {
			t.Fatalf("uid auth response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsMultisearchCommandWithoutCapability(t *testing.T) {
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
	greeting, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if !strings.Contains(greeting, " ESEARCH ") {
		t.Fatalf("greeting missing RFC 4731 ESEARCH capability: %q", greeting)
	}
	if strings.Contains(greeting, " MULTISEARCH ") {
		t.Fatalf("greeting advertised unsupported RFC 7377 MULTISEARCH capability: %q", greeting)
	}
	if _, err := client.Write([]byte("a1 ESEARCH RETURN (COUNT) ALL\r\na2 UID ESEARCH RETURN (COUNT) ALL\r\na3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write ESEARCH commands: %v", err)
	}
	want := []string{
		"a1 BAD ESEARCH command requires MULTISEARCH capability\r\n",
		"a2 BAD ESEARCH command requires MULTISEARCH capability\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a3 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read ESEARCH response: %v", err)
		}
		if line != expected {
			t.Fatalf("ESEARCH response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerValidatesSelectedCommandSyntaxBeforeSelectedState(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 FETCH\r\na3 STORE\r\na4 STORE 1 +FLAGS \\Seen)\r\na5 FETCH +1 (FLAGS)\r\na6 STORE +1 +FLAGS (\\Seen)\r\na7 COPY +1 Archive\r\na8 MOVE +1 Archive\r\na9 COPY 1\r\na10 COPY 1 &Jjo!\r\na11 MOVE 1\r\na12 FETCH \"1: 2\" (FLAGS)\r\na13 SEARCH\r\na14 SEARCH RETURN (COUNT COUNT) ALL\r\na15 SEARCH CHARSET UTF-8\r\na16 SORT\r\na17 SORT (DATE) UTF-8\r\na18 THREAD\r\na19 THREAD REFERENCES UTF-8 ALL\r\na20 FETCH 1 ((FLAGS))\r\na21 STORE 1 +FLAGS (\\Seen \\Seen)\r\na22 FETCH 1 (FLAGS)\r\na23 LOGOUT\r\n")); err != nil {
		t.Fatalf("write selected-state commands: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a2 BAD FETCH requires sequence set and data items\r\n",
		"a3 BAD STORE requires sequence set, mode, and flags\r\n",
		"a4 BAD STORE flags are unsupported\r\n",
		"a5 BAD FETCH requires a valid message sequence set\r\n",
		"a6 BAD STORE requires a valid message sequence set\r\n",
		"a7 BAD COPY requires a valid message sequence set\r\n",
		"a8 BAD MOVE requires a valid message sequence set\r\n",
		"a9 BAD COPY requires sequence set and destination mailbox\r\n",
		"a10 BAD COPY destination mailbox name is not valid modified UTF-7\r\n",
		"a11 BAD MOVE requires sequence set and destination mailbox\r\n",
		"a12 BAD FETCH requires a valid message sequence set\r\n",
		"a13 BAD SEARCH requires criteria\r\n",
		"a14 BAD SEARCH return options are unsupported\r\n",
		"a15 BAD SEARCH requires criteria\r\n",
		"a16 BAD SORT requires sort criteria, charset, and search criteria\r\n",
		"a17 BAD SORT requires sort criteria, charset, and search criteria\r\n",
		"a18 BAD THREAD requires algorithm, charset, and search criteria\r\n",
		"a19 BAD THREAD algorithm is unsupported\r\n",
		"a20 BAD FETCH data item list is invalid\r\n",
		"a21 BAD STORE flags are unsupported\r\n",
		"a22 NO mailbox must be selected\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a23 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read selected-state response: %v", err)
		}
		if line != expected {
			t.Fatalf("selected-state response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsQuotedSequenceSetArgumentsBeforeState(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 FETCH \"1\" (FLAGS)\r\na3 COPY \"1\" Archive\r\na4 MOVE \"1\" Archive\r\na5 UID FETCH \"7\" (FLAGS)\r\na6 UID COPY \"7\" Archive\r\na7 UID MOVE \"7\" Archive\r\na8 UID EXPUNGE \"7\"\r\na9 LOGOUT\r\n")); err != nil {
		t.Fatalf("write quoted sequence-set commands: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a2 BAD FETCH requires a valid message sequence set\r\n",
		"a3 BAD COPY requires a valid message sequence set\r\n",
		"a4 BAD MOVE requires a valid message sequence set\r\n",
		"a5 BAD UID FETCH requires a positive UID set\r\n",
		"a6 BAD UID COPY requires a positive UID set\r\n",
		"a7 BAD UID MOVE requires a positive UID set\r\n",
		"a8 BAD UID EXPUNGE requires a positive UID set\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a9 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read quoted sequence-set response: %v", err)
		}
		if line != expected {
			t.Fatalf("quoted sequence-set response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsStringCommandAtoms(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 \"NOOP\"\r\na2 {4}\r\n")); err != nil {
		t.Fatalf("write quoted command atoms: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 BAD malformed command\r\n" {
		t.Fatalf("quoted command response = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("literal command continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("NOOP\r\n")); err != nil {
		t.Fatalf("write literal command atom: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 BAD malformed command\r\n" {
		t.Fatalf("literal command response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a3 LOGIN user@example.com secret\r\na4 UID \"COPY\" 7 Archive\r\na5 UID {4+}\r\nCOPY 7 Archive\r\na6 LOGOUT\r\n")); err != nil {
		t.Fatalf("write uid string command atoms: %v", err)
	}
	want := []string{
		"a3 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a4 BAD malformed command\r\n",
		"a5 BAD malformed command\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a6 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read string command atom response: %v", err)
		}
		if line != expected {
			t.Fatalf("string command atom response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsStringCommandTags(t *testing.T) {
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
	if _, err := client.Write([]byte("\"a1\" NOOP\r\n{2}\r\n")); err != nil {
		t.Fatalf("write string command tags: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* BAD malformed command\r\n" {
		t.Fatalf("quoted tag response = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("literal tag continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 NOOP\r\n")); err != nil {
		t.Fatalf("write literal tag body: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* BAD malformed command\r\n" {
		t.Fatalf("literal tag response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	want := []string{
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a3 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read logout response: %v", err)
		}
		if line != expected {
			t.Fatalf("logout response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsLiteralSequenceSetArgumentsBeforeState(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 COPY {1}\r\n")); err != nil {
		t.Fatalf("write copy literal sequence marker: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("copy literal continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("1 Archive\r\n")); err != nil {
		t.Fatalf("write copy literal sequence suffix: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 BAD COPY requires a valid message sequence set\r\n" {
		t.Fatalf("copy literal sequence response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a3 UID MOVE {1+}\r\n7 Archive\r\n")); err != nil {
		t.Fatalf("write uid move literal+ sequence: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 BAD UID MOVE requires a positive UID set\r\n" {
		t.Fatalf("uid move literal+ sequence response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	want := []string{
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a4 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read logout response: %v", err)
		}
		if line != expected {
			t.Fatalf("logout response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsStringSearchSequenceSetArguments(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SEARCH {1}\r\n")); err != nil {
		t.Fatalf("write search literal marker: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("search literal continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("1\r\n")); err != nil {
		t.Fatalf("write search literal sequence-set: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 BAD SEARCH criteria are unsupported\r\n" {
		t.Fatalf("search literal sequence-set response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a3 UID SEARCH UID {1+}\r\n7\r\n")); err != nil {
		t.Fatalf("write uid search literal+ sequence-set: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 BAD SEARCH criteria are unsupported\r\n" {
		t.Fatalf("uid search literal+ sequence-set response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	want := []string{
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a4 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read logout response: %v", err)
		}
		if line != expected {
			t.Fatalf("logout response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsLiteralSortThreadSequenceSetArguments(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 SORT (DATE) UTF-8 {1}\r\n")); err != nil {
		t.Fatalf("write sort literal marker: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("sort literal continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("1\r\n")); err != nil {
		t.Fatalf("write sort literal set: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 BAD SORT criteria are unsupported\r\n" {
		t.Fatalf("sort literal response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 THREAD ORDEREDSUBJECT UTF-8 {1+}\r\n1\r\n")); err != nil {
		t.Fatalf("write thread literal+ set: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 BAD THREAD criteria are unsupported\r\n" {
		t.Fatalf("thread literal response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	want := []string{
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a3 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read logout response: %v", err)
		}
		if line != expected {
			t.Fatalf("logout response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsStringSortThreadNumericSearchArguments(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 SORT (DATE) UTF-8 LARGER \"20\"\r\na2 UID SORT (DATE) UTF-8 MODSEQ \"20\"\r\na3 THREAD ORDEREDSUBJECT UTF-8 SMALLER \"20\"\r\na4 UID THREAD ORDEREDSUBJECT UTF-8 MODSEQ \"/flags/\\\\Seen\" \"all\" 17\r\na5 LOGOUT\r\n")); err != nil {
		t.Fatalf("write sort/thread numeric string arguments: %v", err)
	}
	want := []string{
		"a1 BAD SORT criteria are unsupported\r\n",
		"a2 BAD SORT criteria are unsupported\r\n",
		"a3 BAD THREAD criteria are unsupported\r\n",
		"a4 BAD THREAD criteria are unsupported\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a5 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read sort/thread numeric string response: %v", err)
		}
		if line != expected {
			t.Fatalf("sort/thread numeric string response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsStringStoreControlAtomsBeforeState(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 STORE 1 \"+FLAGS\" (\\Seen)\r\na2 UID STORE 7 {6+}\r\n+FLAGS (\\Seen)\r\na3 STORE 1 \"(UNCHANGEDSINCE\" 27) +FLAGS (\\Seen)\r\na4 UID STORE 7 (UNCHANGEDSINCE 27) \"+FLAGS\" (\\Seen)\r\na5 LOGOUT\r\n")); err != nil {
		t.Fatalf("write string store controls: %v", err)
	}
	want := []string{
		"a1 BAD STORE mode is unsupported\r\n",
		"a2 BAD UID STORE mode is unsupported\r\n",
		"a3 BAD STORE UNCHANGEDSINCE modifier is invalid\r\n",
		"a4 BAD UID STORE mode is unsupported\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a5 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read string store control response: %v", err)
		}
		if line != expected {
			t.Fatalf("string store control response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsStringStoreFlagListsBeforeState(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 STORE 1 +FLAGS \"(\\\\Seen)\"\r\na2 UID STORE 7 +FLAGS \"(\\\\Seen)\"\r\na3 STORE 1 +FLAGS {7+}\r\n(\\Seen)\r\na4 UID STORE 7 +FLAGS {7+}\r\n(\\Seen)\r\na5 STORE 1 +FLAGS (\\Seen)\r\na6 LOGOUT\r\n")); err != nil {
		t.Fatalf("write string store flag lists: %v", err)
	}
	want := []string{
		"a1 BAD STORE flags are unsupported\r\n",
		"a2 BAD UID STORE flags are unsupported\r\n",
		"a3 BAD STORE flags are unsupported\r\n",
		"a4 BAD UID STORE flags are unsupported\r\n",
		"a5 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a6 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read string store flag-list response: %v", err)
		}
		if line != expected {
			t.Fatalf("string store flag-list response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsUnsupportedFetchDataItemsBeforeMailboxState(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 FETCH 1 BOGUS\r\na2 FETCH 1 \"FLAGS\"\r\na3 UID FETCH 7 {5+}\r\nFLAGS\r\na4 LOGIN user@example.com secret\r\na5 FETCH 1 (FLAGS BOGUS)\r\na6 FETCH 1 (FLAGS)\r\na7 LOGOUT\r\n")); err != nil {
		t.Fatalf("write unsupported fetch commands: %v", err)
	}
	want := []string{
		"a1 BAD FETCH data item is unsupported\r\n",
		"a2 BAD FETCH data item is unsupported\r\n",
		"a3 BAD FETCH data item is unsupported\r\n",
		"a4 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a5 BAD FETCH data item is unsupported\r\n",
		"a6 NO mailbox must be selected\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a7 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read unsupported fetch response: %v", err)
		}
		if line != expected {
			t.Fatalf("unsupported fetch response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerValidatesSelectedActionSyntaxBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 FETCH\r\na2 STORE\r\na3 COPY 1\r\na4 COPY 1 &Jjo!\r\na5 MOVE 1\r\na6 MOVE 1 &Jjo!\r\na7 FETCH +1 (FLAGS)\r\na8 STORE +1 +FLAGS (\\Seen)\r\na9 COPY +1 Archive\r\na10 MOVE +1 Archive\r\na11 FETCH 1 (FLAGS)\r\na12 FETCH 1 ((FLAGS))\r\na13 STORE 1 +FLAGS \\Seen)\r\na14 STORE 1 +FLAGS (\\Seen)\r\na15 COPY 1 Archive\r\na16 MOVE 1 Archive\r\na17 LOGOUT\r\n")); err != nil {
		t.Fatalf("write selected action auth commands: %v", err)
	}
	want := []string{
		"a1 BAD FETCH requires sequence set and data items\r\n",
		"a2 BAD STORE requires sequence set, mode, and flags\r\n",
		"a3 BAD COPY requires sequence set and destination mailbox\r\n",
		"a4 BAD COPY destination mailbox name is not valid modified UTF-7\r\n",
		"a5 BAD MOVE requires sequence set and destination mailbox\r\n",
		"a6 BAD MOVE destination mailbox name is not valid modified UTF-7\r\n",
		"a7 BAD FETCH requires a valid message sequence set\r\n",
		"a8 BAD STORE requires a valid message sequence set\r\n",
		"a9 BAD COPY requires a valid message sequence set\r\n",
		"a10 BAD MOVE requires a valid message sequence set\r\n",
		"a11 NO authentication required\r\n",
		"a12 BAD FETCH data item list is invalid\r\n",
		"a13 BAD STORE flags are unsupported\r\n",
		"a14 NO authentication required\r\n",
		"a15 NO authentication required\r\n",
		"a16 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a17 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read selected action auth response: %v", err)
		}
		if line != expected {
			t.Fatalf("selected action auth response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerValidatesAppendSyntaxBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 APPEND\r\na2 APPEND inbox BAD\r\na3 APPEND &Jjo! {5+}\r\nhello\r\na4 APPEND inbox BAD {5+}\r\nhello\r\na5 APPEND inbox (\\Seen {5+}\r\nhello\r\na6 APPEND inbox \"5-May-2026 12:34:56 +0900\" {5+}\r\nhello\r\na7 APPEND inbox {5+}\r\nhello\r\na8 LOGOUT\r\n")); err != nil {
		t.Fatalf("write append auth commands: %v", err)
	}
	want := []string{
		"a1 BAD APPEND requires mailbox and literal\r\n",
		"a2 BAD APPEND requires mailbox and literal\r\n",
		"a3 BAD APPEND mailbox name is not valid modified UTF-7\r\n",
		"a4 BAD APPEND options are unsupported\r\n",
		"a5 BAD APPEND options are unsupported\r\n",
		"a6 BAD APPEND options are unsupported\r\n",
		"a7 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a8 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read append auth response: %v", err)
		}
		if line != expected {
			t.Fatalf("append auth response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerValidatesSearchSyntaxBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 SEARCH\r\na2 SEARCH RETURN (COUNT COUNT) ALL\r\na3 SORT\r\na4 SORT DATE UTF-8 ALL\r\na5 SORT (DATE) UTF-8\r\na6 THREAD\r\na7 THREAD REFERENCES UTF-8 ALL\r\na8 SEARCH CHARSET UTF-8\r\na9 SEARCH +1\r\na10 UID SEARCH UID +7\r\na11 SEARCH HEADER \"\" value\r\na12 SEARCH HEADER \"Bad Field\" value\r\na13 SEARCH HEADER Subject: value\r\na14 SEARCH X-GM-RAW test\r\na15 SEARCH ALL\r\na16 SORT (DATE) UTF-8 +1\r\na17 THREAD ORDEREDSUBJECT UTF-8 +1\r\na18 SORT (DATE) UTF-8 \"1\"\r\na19 UID SORT (DATE) UTF-8 \"7\"\r\na20 THREAD ORDEREDSUBJECT UTF-8 \"1\"\r\na21 UID THREAD ORDEREDSUBJECT UTF-8 \"7\"\r\na22 SEARCH CHARSET \"UTF-8\" ALL\r\na23 SORT (DATE) \"UTF-8\" ALL\r\na24 THREAD ORDEREDSUBJECT \"UTF-8\" ALL\r\na25 SEARCH KEYWORD \"custom\"\r\na26 UID SEARCH UNKEYWORD \"custom\"\r\na27 SORT (DATE) UTF-8 ALL\r\na28 THREAD ORDEREDSUBJECT UTF-8 ALL\r\na29 SEARCH CHARSET ISO-8859-1 ALL\r\na30 SORT (DATE) ISO-8859-1 ALL\r\na31 THREAD ORDEREDSUBJECT ISO-8859-1 ALL\r\na32 LOGOUT\r\n")); err != nil {
		t.Fatalf("write search auth commands: %v", err)
	}
	want := []string{
		"a1 BAD SEARCH requires criteria\r\n",
		"a2 BAD SEARCH return options are unsupported\r\n",
		"a3 BAD SORT requires sort criteria, charset, and search criteria\r\n",
		"a4 BAD SORT arguments are unsupported\r\n",
		"a5 BAD SORT requires sort criteria, charset, and search criteria\r\n",
		"a6 BAD THREAD requires algorithm, charset, and search criteria\r\n",
		"a7 BAD THREAD algorithm is unsupported\r\n",
		"a8 BAD SEARCH requires criteria\r\n",
		"a9 BAD SEARCH criteria are unsupported\r\n",
		"a10 BAD SEARCH criteria are unsupported\r\n",
		"a11 BAD SEARCH criteria are unsupported\r\n",
		"a12 BAD SEARCH criteria are unsupported\r\n",
		"a13 BAD SEARCH criteria are unsupported\r\n",
		"a14 BAD SEARCH criteria are unsupported\r\n",
		"a15 NO authentication required\r\n",
		"a16 BAD SORT criteria are unsupported\r\n",
		"a17 BAD THREAD criteria are unsupported\r\n",
		"a18 BAD SORT criteria are unsupported\r\n",
		"a19 BAD SORT criteria are unsupported\r\n",
		"a20 BAD THREAD criteria are unsupported\r\n",
		"a21 BAD THREAD criteria are unsupported\r\n",
		"a22 BAD SEARCH criteria are unsupported\r\n",
		"a23 BAD SORT arguments are unsupported\r\n",
		"a24 BAD THREAD arguments are unsupported\r\n",
		"a25 BAD SEARCH criteria are unsupported\r\n",
		"a26 BAD SEARCH criteria are unsupported\r\n",
		"a27 NO authentication required\r\n",
		"a28 NO authentication required\r\n",
		"a29 NO [BADCHARSET (US-ASCII UTF-8)] SEARCH charset is unsupported\r\n",
		"a30 NO [BADCHARSET (US-ASCII UTF-8)] SORT charset is unsupported\r\n",
		"a31 NO [BADCHARSET (US-ASCII UTF-8)] THREAD charset is unsupported\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a32 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read search auth response: %v", err)
		}
		if line != expected {
			t.Fatalf("search auth response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerValidatesSelectedNoArgSyntaxBeforeSelectedState(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 CHECK extra\r\na3 IDLE extra\r\na4 CLOSE extra\r\na5 UNSELECT extra\r\na6 EXPUNGE 1:*\r\na7 CHECK\r\na8 LOGOUT\r\n")); err != nil {
		t.Fatalf("write selected no-arg commands: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a2 BAD CHECK does not accept arguments\r\n",
		"a3 BAD IDLE does not accept arguments\r\n",
		"a4 BAD CLOSE does not accept arguments\r\n",
		"a5 BAD UNSELECT does not accept arguments\r\n",
		"a6 BAD EXPUNGE does not accept arguments\r\n",
		"a7 NO mailbox must be selected\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a8 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read selected no-arg response: %v", err)
		}
		if line != expected {
			t.Fatalf("selected no-arg response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerValidatesSelectedNoArgSyntaxBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 CHECK extra\r\na2 IDLE extra\r\na3 CLOSE extra\r\na4 UNSELECT extra\r\na5 EXPUNGE 1:*\r\na6 CHECK\r\na7 IDLE\r\na8 CLOSE\r\na9 UNSELECT\r\na10 EXPUNGE\r\na11 LOGOUT\r\n")); err != nil {
		t.Fatalf("write selected no-arg auth commands: %v", err)
	}
	want := []string{
		"a1 BAD CHECK does not accept arguments\r\n",
		"a2 BAD IDLE does not accept arguments\r\n",
		"a3 BAD CLOSE does not accept arguments\r\n",
		"a4 BAD UNSELECT does not accept arguments\r\n",
		"a5 BAD EXPUNGE does not accept arguments\r\n",
		"a6 NO authentication required\r\n",
		"a7 NO authentication required\r\n",
		"a8 NO authentication required\r\n",
		"a9 NO authentication required\r\n",
		"a10 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a11 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read selected no-arg auth response: %v", err)
		}
		if line != expected {
			t.Fatalf("selected no-arg auth response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsStringAppendFlagListsBeforeState(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 APPEND inbox \"(\\\\Seen)\" {5+}\r\nhello\r\na2 APPEND inbox {7+}\r\n(\\Seen) {5+}\r\nhello\r\na3 APPEND inbox (\\Seen \\Seen) {5+}\r\nhello\r\na4 APPEND inbox (\\Seen) {5+}\r\nhello\r\na5 LOGOUT\r\n")); err != nil {
		t.Fatalf("write string append flag lists: %v", err)
	}
	want := []string{
		"a1 BAD APPEND options are unsupported\r\n",
		"a2 BAD APPEND options are unsupported\r\n",
		"a3 BAD APPEND options are unsupported\r\n",
		"a4 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a5 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read string append flag-list response: %v", err)
		}
		if line != expected {
			t.Fatalf("string append flag-list response = %q, want %q", line, expected)
		}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "* OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT STARTTLS LOGINDISABLED] gogomail IMAP4rev1 service ready\r\n" {
		t.Fatalf("greeting = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a1 CAPABILITY\r\n")); err != nil {
		t.Fatalf("write capability: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT STARTTLS LOGINDISABLED\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT SASL-IR AUTH=PLAIN] Begin TLS negotiation now\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT SASL-IR AUTH=PLAIN\r\n" {
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

func TestServerImplicitTLSGreetingAdvertisesAuthenticatedLogin(t *testing.T) {
	t.Parallel()

	serverTLS := testIMAPTLSConfig(t)
	server, err := NewServer(ServerOptions{Addr: ":1993", Backend: fakeBackend{}, TLSConfig: serverTLS})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	tlsBackend := tls.Server(backend, serverTLS)
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(tlsBackend)
	}()

	tlsClient := tls.Client(client, &tls.Config{InsecureSkipVerify: true})
	defer tlsClient.Close()
	reader := bufio.NewReader(tlsClient)
	if line, err := reader.ReadString('\n'); err != nil || line != "* OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT SASL-IR AUTH=PLAIN] gogomail IMAP4rev1 service ready\r\n" {
		t.Fatalf("implicit TLS greeting = %q err = %v", line, err)
	}
	if _, err := tlsClient.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LOGOUT\r\n")); err != nil {
		t.Fatalf("write implicit tls login: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a2 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read implicit tls response: %v", err)
		}
		if line != expected {
			t.Fatalf("implicit tls response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerValidatesAuthSyntaxBeforePrivacyRequired(t *testing.T) {
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
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com\r\na2 AUTHENTICATE \"BOGUS\"\r\na3 AUTHENTICATE \"PLAIN\"\r\na4 AUTHENTICATE BOGUS\r\na5 AUTHENTICATE PLAIN \"AHVzZXJAZXhhbXBsZS5jb20Ac2VjcmV0\"\r\na6 AUTHENTICATE PLAIN not-base64\r\na7 AUTHENTICATE PLAIN\r\na8 LOGIN user@example.com secret\r\na9 LOGOUT\r\n")); err != nil {
		t.Fatalf("write auth commands: %v", err)
	}
	want := []string{
		"a1 BAD LOGIN requires username and password atoms\r\n",
		"a2 BAD AUTHENTICATE mechanism is malformed\r\n",
		"a3 BAD AUTHENTICATE mechanism is malformed\r\n",
		"a4 NO AUTHENTICATE mechanism is unsupported\r\n",
		"a5 BAD AUTHENTICATE PLAIN response is malformed\r\n",
		"a6 NO [PRIVACYREQUIRED] TLS is required for AUTHENTICATE\r\n",
		"a7 NO [PRIVACYREQUIRED] TLS is required for AUTHENTICATE\r\n",
		"a8 NO [PRIVACYREQUIRED] TLS is required for LOGIN\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a9 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read auth response: %v", err)
		}
		if line != expected {
			t.Fatalf("auth response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsUnsupportedAuthenticateMechanismWithNo(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 AUTHENTICATE SCRAM-SHA-256\r\na2 LOGOUT\r\n")); err != nil {
		t.Fatalf("write authenticate/logout: %v", err)
	}
	want := []string{
		"a1 NO AUTHENTICATE mechanism is unsupported\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a2 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read response: %v", err)
		}
		if line != expected {
			t.Fatalf("response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerValidatesStartTLSArgumentsBeforeAvailability(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 STARTTLS extra\r\na2 STARTTLS\r\na3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write starttls commands: %v", err)
	}
	want := []string{
		"a1 BAD STARTTLS does not accept arguments\r\n",
		"a2 BAD STARTTLS is unavailable\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a3 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read starttls response: %v", err)
		}
		if line != expected {
			t.Fatalf("starttls response = %q, want %q", line, expected)
		}
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
	if line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login = %q", line)
	}
	if _, err := client.Write([]byte("a2 CAPABILITY\r\n")); err != nil {
		t.Fatalf("write authenticated capability: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read authenticated capability untagged: %v", err)
	}
	if line != "* CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT\r\n" {
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

func TestServerLoginAcceptsMultipleSynchronizingLiterals(t *testing.T) {
	t.Parallel()

	authBackend := literalLoginBackend{creds: make(chan [2]string, 1)}
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: authBackend, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN {16}\r\n")); err != nil {
		t.Fatalf("write login literal command: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("first literal continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("user@example.com {6}\r\n")); err != nil {
		t.Fatalf("write login first literal: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("second literal continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("secret\r\n")); err != nil {
		t.Fatalf("write login second literal: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("literal login = %q err = %v", line, err)
	}
	select {
	case got := <-authBackend.creds:
		if got != [2]string{"user@example.com", "secret"} {
			t.Fatalf("Authenticate credentials = %#v", got)
		}
	default:
		t.Fatalf("Authenticate was not called")
	}
	if _, err := client.Write([]byte("a2 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* BYE gogomail IMAP4rev1 server logging out\r\n" {
		t.Fatalf("logout bye = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 OK LOGOUT completed\r\n" {
		t.Fatalf("logout completion = %q err = %v", line, err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerLoginFailureIncludesAuthenticationFailedCode(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: authFailureBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com wrong\r\na2 LOGOUT\r\n")); err != nil {
		t.Fatalf("write login/logout: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 NO [AUTHENTICATIONFAILED] LOGIN failed\r\n" {
		t.Fatalf("login failure = %q err = %v", line, err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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

func TestServerHandlesIDParameterList(t *testing.T) {
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
	if _, err := client.Write([]byte(`a1 ID ("name" "gogomail test" "version" NIL)` + "\r\n")); err != nil {
		t.Fatalf("write id list: %v", err)
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

func TestServerHandlesIDParameterListLiterals(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 ID (\"name\" {13}\r\n")); err != nil {
		t.Fatalf("write id literal command: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("literal continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("gogomail test \"version\" {5+}\r\n1.2.3)\r\n")); err != nil {
		t.Fatalf("write id literal body: %v", err)
	}
	want := []string{
		"* ID (\"name\" \"gogomail\")\r\n",
		"a1 OK ID completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read id literal response: %v", err)
		}
		if line != expected {
			t.Fatalf("id literal response = %q, want %q", line, expected)
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

func TestServerHandlesBareIDCommand(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 ID\r\na2 LOGOUT\r\n")); err != nil {
		t.Fatalf("write bare id command: %v", err)
	}
	want := []string{
		"* ID (\"name\" \"gogomail\")\r\n",
		"a1 OK ID completed\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a2 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read bare id response: %v", err)
		}
		if line != expected {
			t.Fatalf("bare id response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsMalformedIDArguments(t *testing.T) {
	t.Parallel()

	for _, command := range []string{
		`ID NIL "extra"`,
		`ID "name" "client"`,
		`ID ("name")`,
		`ID ("name""client")`,
		`ID ("name" bad\client)`,
		`ID ("name" "client" "name" "duplicate")`,
		`ID ("0123456789012345678901234567890" "client")`,
	} {
		command := command
		t.Run(command, func(t *testing.T) {
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
			if _, err := client.Write([]byte("a1 " + command + "\r\na2 LOGOUT\r\n")); err != nil {
				t.Fatalf("write id/logout: %v", err)
			}
			if line, err := reader.ReadString('\n'); err != nil || line != "a1 BAD ID requires NIL or parameter list\r\n" {
				t.Fatalf("id response = %q err = %v", line, err)
			}
			_, _ = reader.ReadString('\n')
			_, _ = reader.ReadString('\n')
			if err := <-errCh; err != nil {
				t.Fatalf("ServeConn returned error: %v", err)
			}
		})
	}
}

func TestIMAPIDArgumentsValidEnforcesRFC2971Limits(t *testing.T) {
	t.Parallel()

	if !imapIDArgumentsValid(`("name" "` + strings.Repeat("x", 1024) + `")`) {
		t.Fatal("imapIDArgumentsValid rejected 1024-octet ID value")
	}
	if imapIDArgumentsValid(`("name" "` + strings.Repeat("x", 1025) + `")`) {
		t.Fatal("imapIDArgumentsValid accepted oversized ID value")
	}

	pairs := make([]string, 0, 62)
	for i := 0; i < 31; i++ {
		pairs = append(pairs, fmt.Sprintf(`"field-%02d" "value"`, i))
	}
	if imapIDArgumentsValid("(" + strings.Join(pairs, " ") + ")") {
		t.Fatal("imapIDArgumentsValid accepted more than 30 ID field-value pairs")
	}
	if imapIDArgumentsValid(`("name" "bad\q")`) {
		t.Fatal("imapIDArgumentsValid accepted unsupported quoted escape")
	}
	if imapIDArgumentsValid("(\"name\" \"bad\x80value\")") {
		t.Fatal("imapIDArgumentsValid accepted non-ascii quoted value")
	}
	if imapIDArgumentsValid(`("name""value")`) {
		t.Fatal("imapIDArgumentsValid accepted adjacent quoted tokens")
	}
	if imapIDArgumentsValid(`("name" bad"value)`) {
		t.Fatal("imapIDArgumentsValid accepted quote inside unquoted value")
	}
	if imapIDArgumentsValid(`("name" bad\value)`) {
		t.Fatal("imapIDArgumentsValid accepted backslash inside unquoted value")
	}
	if imapIDArgumentsValid(`("name" bad{value)`) {
		t.Fatal("imapIDArgumentsValid accepted literal marker inside unquoted value")
	}
	if imapIDArgumentsValid(`("name" bad]value)`) {
		t.Fatal("imapIDArgumentsValid accepted response-special inside unquoted value")
	}
	if imapIDArgumentsValid(`("name" bad*value)`) {
		t.Fatal("imapIDArgumentsValid accepted wildcard atom-special inside unquoted value")
	}
	if !imapIDArgumentsValid(`("name" "Project \"Q2\"")`) {
		t.Fatal("imapIDArgumentsValid rejected escaped quoted-special")
	}
	if !imapIDArgumentsValidWithLiterals(`("name" {5} "version" {3})`, []string{"Apple", "1.0"}) {
		t.Fatal("imapIDArgumentsValidWithLiterals rejected literal ID strings")
	}
	if imapIDArgumentsValidWithLiterals(`("name" {5})`, nil) {
		t.Fatal("imapIDArgumentsValidWithLiterals accepted missing literal")
	}
	if imapIDArgumentsValidWithLiterals(`("name" "client")`, []string{"unused"}) {
		t.Fatal("imapIDArgumentsValidWithLiterals accepted unused literal")
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerPreservesQuotedLoginCredentialSpaces(t *testing.T) {
	t.Parallel()

	authBackend := literalLoginBackend{creds: make(chan [2]string, 1)}
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: authBackend, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN \" user@example.com \" \" secret \"\r\na2 LOGOUT\r\n")); err != nil {
		t.Fatalf("write login/logout: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	select {
	case got := <-authBackend.creds:
		if got != [2]string{" user@example.com ", " secret "} {
			t.Fatalf("Authenticate credentials = %#v", got)
		}
	default:
		t.Fatalf("Authenticate was not called")
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerTreatsEmptyLoginPasswordAsAuthenticationFailure(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: authFailureBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com \"\"\r\na2 LOGOUT\r\n")); err != nil {
		t.Fatalf("write login/logout: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 NO [AUTHENTICATIONFAILED] LOGIN failed\r\n" {
		t.Fatalf("empty-password login = %q err = %v", line, err)
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
	if line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] AUTHENTICATE completed\r\n" {
		t.Fatalf("authenticate completion = %q", line)
	}
	if _, err := client.Write([]byte("a2 CAPABILITY\r\n")); err != nil {
		t.Fatalf("write capability: %v", err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT\r\n" {
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

func TestServerAuthenticatePlainCancelReturnsBad(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "+ \r\n" {
		t.Fatalf("continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("*\r\n")); err != nil {
		t.Fatalf("write authenticate cancel: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 BAD AUTHENTICATE canceled\r\n" {
		t.Fatalf("authenticate cancel completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 AUTHENTICATE PLAIN\r\n")); err != nil {
		t.Fatalf("write second authenticate: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ \r\n" {
		t.Fatalf("second continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte(" *\r\n")); err != nil {
		t.Fatalf("write space-padded authenticate cancel: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 BAD AUTHENTICATE PLAIN response is malformed\r\n" {
		t.Fatalf("space-padded cancel response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a3 CAPABILITY\r\n")); err != nil {
		t.Fatalf("write capability: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT SASL-IR AUTH=PLAIN\r\n" {
		t.Fatalf("post-cancel capability = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 OK CAPABILITY completed\r\n" {
		t.Fatalf("post-cancel capability completion = %q err = %v", line, err)
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

func TestServerRejectsLFOnlyAuthenticateContinuationBeforeClosing(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "+ \r\n" {
		t.Fatalf("continuation = %q err = %v", line, err)
	}
	response := base64.StdEncoding.EncodeToString([]byte("\x00user@example.com\x00secret"))
	if _, err := client.Write([]byte(response + "\n")); err != nil {
		t.Fatalf("write lf-only authenticate response: %v", err)
	}
	want := []string{
		"a1 BAD command line must end with CRLF\r\n",
		"* BYE gogomail IMAP4rev1 server closing connection after framing error\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read lf-only authenticate response: %v", err)
		}
		if line != expected {
			t.Fatalf("lf-only authenticate response = %q, want %q", line, expected)
		}
	}
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
	if line, err = reader.ReadString('\n'); err != nil || line != "a2 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)] Permanent flags\r\n",
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

func TestServerCanonicalizesPermanentFlagsInSelectResponses(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: duplicatePermanentFlagsBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\na3 STORE 1 +FLAGS (Forwarded)\r\n")); err != nil {
		t.Fatalf("write login/select/store: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* FLAGS (\\Seen $Forwarded \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [PERMANENTFLAGS (\\Seen $Forwarded \\Deleted)] Permanent flags\r\n",
		"a2 OK [READ-WRITE] SELECT completed\r\n",
		"* 1 FETCH (UID 7 FLAGS ($Forwarded))\r\n",
		"a3 OK STORE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read response: %v", err)
		}
		if line != expected {
			t.Fatalf("response = %q, want %q", line, expected)
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

func TestServerFailedReselectPreservesCurrentMailbox(t *testing.T) {
	t.Parallel()

	backendImpl := &selectMissingAfterSelectBackend{}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\na3 SELECT missing\r\na4 FETCH 1 (FLAGS)\r\n")); err != nil {
		t.Fatalf("write login/select/fetch: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)] Permanent flags\r\n",
		"a2 OK [READ-WRITE] SELECT completed\r\n",
		"a3 NO [NONEXISTENT] SELECT mailbox does not exist\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"a4 OK FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read response: %v", err)
		}
		if line != expected {
			t.Fatalf("response = %q, want %q", line, expected)
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
	if backendImpl.canceled != 1 {
		t.Fatalf("subscription cancel count = %d, want 1", backendImpl.canceled)
	}
}

func TestServerSelectEmitsFirstUnseenSequence(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: unseenSelectBackend{}, AllowInsecureAuth: true})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UNSEEN 2] Message 2 is first unseen\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)] Permanent flags\r\n",
		"a2 OK [READ-WRITE] SELECT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read select response: %v", err)
		}
		if line != expected {
			t.Fatalf("select response = %q, want %q", line, expected)
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

func TestServerSelectReportsHighestModSeq(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: modSeqBackend{}, AllowInsecureAuth: true})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [HIGHESTMODSEQ 9] Highest mod-sequence\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)] Permanent flags\r\n",
		"a2 OK [READ-WRITE] SELECT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read select response: %v", err)
		}
		if line != expected {
			t.Fatalf("select response = %q, want %q", line, expected)
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

func TestServerSelectReportsUIDNotSticky(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: uidNotStickyBackend{}, AllowInsecureAuth: true})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [UIDNOTSTICKY] UIDs are not sticky\r\n",
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
	if _, err := client.Write([]byte("a3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerSelectCondstoreEnablesModSeqEvents(t *testing.T) {
	t.Parallel()

	backendImpl := &eventBackend{events: make(chan MailboxEvent, 2)}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox (CONDSTORE)\r\n")); err != nil {
		t.Fatalf("write login/select condstore: %v", err)
	}
	wantSelect := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [NOMODSEQ] No persistent mod-sequences\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)] Permanent flags\r\n",
		"a2 OK [READ-WRITE] SELECT completed\r\n",
	}
	for _, expected := range wantSelect {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read login/select condstore response: %v", err)
		}
		if line != expected {
			t.Fatalf("select condstore response = %q, want %q", line, expected)
		}
	}
	backendImpl.events <- MailboxEvent{Type: MailboxEventFlags, UserID: "user-1", MailboxID: "inbox", UID: 7}
	if _, err := client.Write([]byte("a3 NOOP\r\n")); err != nil {
		t.Fatalf("write noop: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) MODSEQ (17))\r\n",
		"a3 OK NOOP completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read noop condstore response: %v", err)
		}
		if line != expected {
			t.Fatalf("noop condstore response = %q, want %q", line, expected)
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

func TestServerEnableCondstoreEnablesModSeqEvents(t *testing.T) {
	t.Parallel()

	backendImpl := &eventBackend{events: make(chan MailboxEvent, 2)}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 ENABLE CONDSTORE X-UNKNOWN\r\na3 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/enable/select: %v", err)
	}
	wantPrefix := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* ENABLED CONDSTORE\r\n",
		"a2 OK ENABLE completed\r\n",
	}
	for _, expected := range wantPrefix {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read enable response: %v", err)
		}
		if line != expected {
			t.Fatalf("enable response = %q, want %q", line, expected)
		}
	}
	wantSelect := []string{
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [NOMODSEQ] No persistent mod-sequences\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)] Permanent flags\r\n",
		"a3 OK [READ-WRITE] SELECT completed\r\n",
	}
	for _, expected := range wantSelect {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read select response: %v", err)
		}
		if line != expected {
			t.Fatalf("select after enable response = %q, want %q", line, expected)
		}
	}
	backendImpl.events <- MailboxEvent{Type: MailboxEventFlags, UserID: "user-1", MailboxID: "inbox", UID: 7}
	if _, err := client.Write([]byte("a4 NOOP\r\n")); err != nil {
		t.Fatalf("write noop: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) MODSEQ (17))\r\n",
		"a4 OK NOOP completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read noop enable response: %v", err)
		}
		if line != expected {
			t.Fatalf("noop enable response = %q, want %q", line, expected)
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

func TestServerEnableCondstoreAfterSelectReturnsHighestModSeq(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: modSeqBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\na3 ENABLE CONDSTORE\r\n")); err != nil {
		t.Fatalf("write login/select/enable: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [HIGHESTMODSEQ 9] Highest mod-sequence\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)] Permanent flags\r\n",
		"a2 OK [READ-WRITE] SELECT completed\r\n",
		"* ENABLED CONDSTORE\r\n",
		"* OK [HIGHESTMODSEQ 9] Highest mod-sequence\r\n",
		"a3 OK ENABLE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read enable after select response: %v", err)
		}
		if line != expected {
			t.Fatalf("enable after select response = %q, want %q", line, expected)
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

func TestServerEnableCondstoreAfterNoModSeqSelectReturnsNoModSeq(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\na3 ENABLE CONDSTORE\r\na4 FETCH 1 (MODSEQ)\r\n")); err != nil {
		t.Fatalf("write login/select/enable: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)] Permanent flags\r\n",
		"a2 OK [READ-WRITE] SELECT completed\r\n",
		"* ENABLED CONDSTORE\r\n",
		"* OK [NOMODSEQ] No persistent mod-sequences\r\n",
		"a3 OK ENABLE completed\r\n",
		"a4 BAD FETCH requires persistent mod-sequences\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read enable after select nomodseq response: %v", err)
		}
		if line != expected {
			t.Fatalf("enable after select nomodseq response = %q, want %q", line, expected)
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

func TestServerRejectsModSeqCommandsAfterNoModSeqSelect(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox (CONDSTORE)\r\na3 FETCH 1 (MODSEQ)\r\na4 UID FETCH 7 (FLAGS) (CHANGEDSINCE 1)\r\na5 SEARCH MODSEQ 1\r\na6 UID SEARCH MODSEQ 1\r\na7 SORT (ARRIVAL) UTF-8 MODSEQ 1\r\na8 UID SORT (ARRIVAL) UTF-8 MODSEQ 1\r\na9 THREAD ORDEREDSUBJECT UTF-8 MODSEQ 1\r\na10 UID THREAD ORDEREDSUBJECT UTF-8 MODSEQ 1\r\na11 UID STORE 7 (UNCHANGEDSINCE 1) +FLAGS (\\Seen)\r\na12 STORE 1 (UNCHANGEDSINCE 1) +FLAGS (\\Seen)\r\n")); err != nil {
		t.Fatalf("write modseq commands: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [NOMODSEQ] No persistent mod-sequences\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)] Permanent flags\r\n",
		"a2 OK [READ-WRITE] SELECT completed\r\n",
		"a3 BAD FETCH requires persistent mod-sequences\r\n",
		"a4 BAD UID FETCH requires persistent mod-sequences\r\n",
		"a5 BAD SEARCH requires persistent mod-sequences\r\n",
		"a6 BAD SEARCH requires persistent mod-sequences\r\n",
		"a7 BAD SORT requires persistent mod-sequences\r\n",
		"a8 BAD SORT requires persistent mod-sequences\r\n",
		"a9 BAD THREAD requires persistent mod-sequences\r\n",
		"a10 BAD THREAD requires persistent mod-sequences\r\n",
		"a11 BAD UID STORE requires persistent mod-sequences\r\n",
		"a12 BAD STORE requires persistent mod-sequences\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read modseq command response: %v", err)
		}
		if line != expected {
			t.Fatalf("modseq command response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a13 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerEnableCondstoreDoesNotRepeatSelectedBaseline(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: modSeqBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox (CONDSTORE)\r\na3 ENABLE CONDSTORE\r\n")); err != nil {
		t.Fatalf("write login/select/enable: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [HIGHESTMODSEQ 9] Highest mod-sequence\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)] Permanent flags\r\n",
		"a2 OK [READ-WRITE] SELECT completed\r\n",
		"* ENABLED CONDSTORE\r\n",
		"a3 OK ENABLE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read enable repeat response: %v", err)
		}
		if line != expected {
			t.Fatalf("enable repeat response = %q, want %q", line, expected)
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

func TestServerEnableCondstoreAfterStatusAndSelectDoesNotRepeatBaseline(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: modSeqBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 STATUS inbox (HIGHESTMODSEQ)\r\na3 SELECT inbox\r\na4 ENABLE CONDSTORE\r\n")); err != nil {
		t.Fatalf("write login/status/select/enable: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* STATUS \"INBOX\" (HIGHESTMODSEQ 9)\r\n",
		"a2 OK STATUS completed\r\n",
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [HIGHESTMODSEQ 9] Highest mod-sequence\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)] Permanent flags\r\n",
		"a3 OK [READ-WRITE] SELECT completed\r\n",
		"* ENABLED CONDSTORE\r\n",
		"a4 OK ENABLE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read status/select/enable response: %v", err)
		}
		if line != expected {
			t.Fatalf("status/select/enable response = %q, want %q", line, expected)
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

func TestServerEnableIgnoresUnsupportedCapabilities(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 ENABLE X-UNKNOWN\r\na3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write enable unknown capability: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* ENABLED\r\n",
		"a2 OK ENABLE completed\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a3 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read enable unknown response: %v", err)
		}
		if line != expected {
			t.Fatalf("enable unknown response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerEnableDeduplicatesCondstoreCapability(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 ENABLE CONDSTORE CONDSTORE condstore\r\na3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write enable duplicate condstore: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* ENABLED CONDSTORE\r\n",
		"a2 OK ENABLE completed\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a3 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read duplicate enable response: %v", err)
		}
		if line != expected {
			t.Fatalf("duplicate enable response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerSelectRejectsUnsupportedParameter(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox (QRESYNC)\r\n")); err != nil {
		t.Fatalf("write login/select unsupported parameter: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 BAD SELECT requires a mailbox atom and optional CONDSTORE parameter\r\n" {
		t.Fatalf("select unsupported parameter = %q err = %v", line, err)
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

func TestServerRejectsStringSelectCondstoreParametersBeforeState(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 SELECT inbox \"(CONDSTORE)\"\r\na2 EXAMINE inbox \"(CONDSTORE)\"\r\na3 SELECT inbox {11+}\r\n(CONDSTORE)\r\na4 EXAMINE inbox {11+}\r\n(CONDSTORE)\r\na5 SELECT inbox (CONDSTORE)\r\na6 LOGOUT\r\n")); err != nil {
		t.Fatalf("write string select condstore parameters: %v", err)
	}
	want := []string{
		"a1 BAD SELECT requires a mailbox atom and optional CONDSTORE parameter\r\n",
		"a2 BAD EXAMINE requires a mailbox atom and optional CONDSTORE parameter\r\n",
		"a3 BAD SELECT requires a mailbox atom and optional CONDSTORE parameter\r\n",
		"a4 BAD EXAMINE requires a mailbox atom and optional CONDSTORE parameter\r\n",
		"a5 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a6 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read string select condstore response: %v", err)
		}
		if line != expected {
			t.Fatalf("string select condstore response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesExamineAsReadOnlySelect(t *testing.T) {
	t.Parallel()

	backend := &selectModeBackend{}
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: backend, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, pipe := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(pipe)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 EXAMINE inbox\r\n")); err != nil {
		t.Fatalf("write login/examine: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
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
	if !backend.readOnly {
		t.Fatal("EXAMINE did not pass ReadOnly to backend selection")
	}
}

func TestServerValidatesMalformedMutationsBeforeReadOnly(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: &selectModeBackend{}, AllowInsecureAuth: true})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read examine response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 STORE\r\na4 UID STORE\r\na5 MOVE 1\r\na6 UID MOVE\r\na7 UID EXPUNGE\r\na8 UID EXPUNGE 0\r\na9 STORE 0 +FLAGS (\\Seen)\r\na10 STORE 1 BOGUS (\\Seen)\r\na11 STORE 1 +FLAGS (\\Bogus)\r\na12 UID STORE 0 +FLAGS (\\Seen)\r\na13 UID STORE 7 BOGUS (\\Seen)\r\na14 UID STORE 7 +FLAGS (\\Bogus)\r\na15 MOVE 0 Archive\r\na16 UID MOVE 0 Archive\r\na17 MOVE 1 &Jjo!\r\na18 UID MOVE 7 &Jjo!\r\na19 STORE 1 +FLAGS (\\Seen)\r\na20 UID STORE 7 +FLAGS (\\Seen)\r\na21 UID EXPUNGE 7\r\na22 MOVE 1 Archive\r\na23 UID MOVE 7 Archive\r\n")); err != nil {
		t.Fatalf("write mutation commands: %v", err)
	}
	want := []string{
		"a3 BAD STORE requires sequence set, mode, and flags\r\n",
		"a4 BAD UID STORE requires UID, mode, and flags\r\n",
		"a5 BAD MOVE requires sequence set and destination mailbox\r\n",
		"a6 BAD UID MOVE requires UID set and destination mailbox\r\n",
		"a7 BAD UID EXPUNGE requires UID set\r\n",
		"a8 BAD UID EXPUNGE requires a positive UID set\r\n",
		"a9 BAD STORE requires a valid message sequence set\r\n",
		"a10 BAD STORE mode is unsupported\r\n",
		"a11 BAD STORE flags are unsupported\r\n",
		"a12 BAD UID STORE requires a positive UID set\r\n",
		"a13 BAD UID STORE mode is unsupported\r\n",
		"a14 BAD UID STORE flags are unsupported\r\n",
		"a15 BAD MOVE requires a valid message sequence set\r\n",
		"a16 BAD UID MOVE requires a positive UID set\r\n",
		"a17 BAD MOVE destination mailbox name is not valid modified UTF-7\r\n",
		"a18 BAD UID MOVE destination mailbox name is not valid modified UTF-7\r\n",
		"a19 NO mailbox is read-only\r\n",
		"a20 NO mailbox is read-only\r\n",
		"a21 NO mailbox is read-only\r\n",
		"a22 NO mailbox is read-only\r\n",
		"a23 NO mailbox is read-only\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read mutation response: %v", err)
		}
		if line != expected {
			t.Fatalf("mutation response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a24 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerSelectUsesCanonicalMailboxID(t *testing.T) {
	t.Parallel()

	backend := &canonicalMailboxBackend{}
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: backend, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, pipe := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(pipe)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT INBOX\r\na3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write select/logout: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
	if backend.selectMailboxID != "INBOX" {
		t.Fatalf("select mailbox id = %q, want wire name", backend.selectMailboxID)
	}
	if backend.subscribeMailboxID != "mailbox-uuid" {
		t.Fatalf("subscribe mailbox id = %q, want canonical id", backend.subscribeMailboxID)
	}
}

func TestServerValidatesEnableSyntaxBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 ENABLE\r\na2 ENABLE CONDSTORE)\r\na3 ENABLE \"CONDSTORE\"\r\na4 ENABLE {9+}\r\nCONDSTORE\r\na5 ENABLE CONDSTORE\r\na6 LOGOUT\r\n")); err != nil {
		t.Fatalf("write enable auth commands: %v", err)
	}
	want := []string{
		"a1 BAD ENABLE requires at least one capability\r\n",
		"a2 BAD ENABLE capability is malformed\r\n",
		"a3 BAD ENABLE capability is malformed\r\n",
		"a4 BAD ENABLE capability is malformed\r\n",
		"a5 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a6 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read enable auth response: %v", err)
		}
		if line != expected {
			t.Fatalf("enable auth response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerSelectFailsBeforeSelectedStateWhenSubscriptionFails(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: failingSubscribeBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\na3 FETCH 1 FLAGS\r\n")); err != nil {
		t.Fatalf("write select/fetch: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a2 NO SELECT failed\r\n",
		"a3 NO mailbox must be selected\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read response: %v", err)
		}
		if line != expected {
			t.Fatalf("response = %q, want %q", line, expected)
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

func TestServerSelectWriteFailureCancelsNewSubscription(t *testing.T) {
	t.Parallel()

	backendImpl := &selectSubscribeCancelBackend{}
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: backendImpl, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	writer := bufio.NewWriterSize(failingWriter{}, 1)
	state := &imapConnState{session: &Session{UserID: "user-1"}}

	done, err := server.handleLineWithLiteral(writer, "a1 SELECT inbox", nil, state)
	if err == nil {
		t.Fatal("handleLineWithLiteral error = nil, want write failure")
	}
	if done {
		t.Fatal("handleLineWithLiteral done = true, want false")
	}
	if !backendImpl.canceled {
		t.Fatal("new SELECT subscription was not canceled after write failure")
	}
	if state.cancelEvents != nil || state.events != nil || state.selectedMailbox != "" {
		t.Fatalf("state after failed SELECT = mailbox %q events %#v cancel nil %t", state.selectedMailbox, state.events, state.cancelEvents == nil)
	}
}

func TestServerExamineFailureUsesExamineCommandName(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: failingSubscribeBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 EXAMINE inbox\r\na3 FETCH 1 FLAGS\r\n")); err != nil {
		t.Fatalf("write examine/fetch: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a2 NO EXAMINE failed\r\n",
		"a3 NO mailbox must be selected\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read response: %v", err)
		}
		if line != expected {
			t.Fatalf("response = %q, want %q", line, expected)
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

func TestServerPreservesSelectedMailboxWhenReselectFails(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: selectMissingBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\na3 SELECT missing\r\na4 CHECK\r\na5 LOGOUT\r\n")); err != nil {
		t.Fatalf("write reselect flow: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)] Permanent flags\r\n",
		"a2 OK [READ-WRITE] SELECT completed\r\n",
		"a3 NO [NONEXISTENT] SELECT mailbox does not exist\r\n",
		"a4 OK CHECK completed\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a5 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read reselect response: %v", err)
		}
		if line != expected {
			t.Fatalf("reselect response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestWriteCopyResponseValidatesDestinationForEmptyUIDSet(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		backend Backend
		dest    MailboxID
		want    string
	}{
		{
			name:    "missing destination",
			backend: missingMailboxBackend{},
			dest:    "missing",
			want:    "a1 NO [TRYCREATE] COPY destination mailbox does not exist\r\n",
		},
		{
			name:    "existing destination",
			backend: fakeBackend{},
			dest:    "archive",
			want:    "a1 OK COPY completed\r\n",
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, err := NewServer(ServerOptions{Addr: ":1143", Backend: tt.backend, AllowInsecureAuth: true})
			if err != nil {
				t.Fatalf("NewServer returned error: %v", err)
			}
			var buf bytes.Buffer
			writer := bufio.NewWriter(&buf)
			state := imapConnState{
				session:         &Session{UserID: "user-1"},
				selectedMailbox: "inbox",
			}

			done, err := server.writeCopyResponse(writer, "a1", &state, nil, tt.dest, "COPY")
			if err != nil {
				t.Fatalf("writeCopyResponse returned error: %v", err)
			}
			if done {
				t.Fatal("writeCopyResponse done = true, want false")
			}
			if err := writer.Flush(); err != nil {
				t.Fatalf("flush response: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Fatalf("response = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestServerReturnsNonexistentForMissingMailboxCommands(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: missingMailboxBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT missing\r\na3 EXAMINE missing\r\na4 STATUS missing (MESSAGES)\r\na5 DELETE missing\r\na6 RENAME missing archive\r\n")); err != nil {
		t.Fatalf("write missing mailbox commands: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a2 NO [NONEXISTENT] SELECT mailbox does not exist\r\n",
		"a3 NO [NONEXISTENT] EXAMINE mailbox does not exist\r\n",
		"a4 NO [NONEXISTENT] STATUS mailbox does not exist\r\n",
		"a5 NO [NONEXISTENT] DELETE mailbox does not exist\r\n",
		"a6 NO [NONEXISTENT] RENAME mailbox does not exist\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read missing mailbox response: %v", err)
		}
		if line != expected {
			t.Fatalf("missing mailbox response = %q, want %q", line, expected)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] AUTHENTICATE completed\r\n" {
		t.Fatalf("authenticate initial response completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 CAPABILITY\r\n")); err != nil {
		t.Fatalf("write capability: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT\r\n" {
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

func TestServerRejectsPaddedAuthenticatePlainInitialResponse(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 AUTHENTICATE PLAIN \" " + response + " \"\r\na2 LOGOUT\r\n")); err != nil {
		t.Fatalf("write padded authenticate initial response: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 BAD AUTHENTICATE PLAIN response is malformed\r\n" {
		t.Fatalf("padded authenticate initial response = %q err = %v", line, err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerAuthenticatePlainFailureIncludesAuthenticationFailedCode(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: authFailureBackend{}, AllowInsecureAuth: true})
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
	response := base64.StdEncoding.EncodeToString([]byte("\x00user@example.com\x00wrong"))
	if _, err := client.Write([]byte("a1 AUTHENTICATE PLAIN " + response + "\r\na2 LOGOUT\r\n")); err != nil {
		t.Fatalf("write authenticate/logout: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 NO [AUTHENTICATIONFAILED] AUTHENTICATE failed\r\n" {
		t.Fatalf("authenticate failure = %q err = %v", line, err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesCheckAndCloseAfterSelect(t *testing.T) {
	t.Parallel()

	backendImpl := &closeBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if backendImpl.expungeCount != 1 || backendImpl.expungeMailboxID != "inbox" || backendImpl.expungeUserID != "user-1" {
		t.Fatalf("close expunge = count %d user %q mailbox %q, want writable selected mailbox expunged", backendImpl.expungeCount, backendImpl.expungeUserID, backendImpl.expungeMailboxID)
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

func TestServerRejectsArgumentsForSelectedStateNoArgCommands(t *testing.T) {
	t.Parallel()

	backendImpl := &closeBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 CHECK extra\r\na4 CLOSE extra\r\na5 UNSELECT extra\r\na6 EXPUNGE 1:*\r\na7 FETCH 1 (FLAGS)\r\n")); err != nil {
		t.Fatalf("write no-arg command arguments: %v", err)
	}
	want := []string{
		"a3 BAD CHECK does not accept arguments\r\n",
		"a4 BAD CLOSE does not accept arguments\r\n",
		"a5 BAD UNSELECT does not accept arguments\r\n",
		"a6 BAD EXPUNGE does not accept arguments\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"a7 OK FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read no-arg command response: %v", err)
		}
		if line != expected {
			t.Fatalf("no-arg command response = %q, want %q", line, expected)
		}
	}
	if backendImpl.expungeCount != 0 {
		t.Fatalf("malformed no-arg commands expunge count = %d, want 0", backendImpl.expungeCount)
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

func TestServerCloseReadOnlyMailboxDoesNotExpunge(t *testing.T) {
	t.Parallel()

	backendImpl := &closeBackend{}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 EXAMINE inbox\r\n")); err != nil {
		t.Fatalf("write login/examine: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read examine response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 CLOSE\r\n")); err != nil {
		t.Fatalf("write close: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 OK CLOSE completed\r\n" {
		t.Fatalf("close line = %q err = %v", line, err)
	}
	if backendImpl.expungeCount != 0 {
		t.Fatalf("read-only close expunge count = %d, want 0", backendImpl.expungeCount)
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

func TestServerCloseClearsSavedSearch(t *testing.T) {
	t.Parallel()

	backendImpl := &closeBackend{}
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: backendImpl, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	var output strings.Builder
	writer := bufio.NewWriter(&output)
	state := &imapConnState{
		session:               &Session{UserID: "user-1"},
		selectedMailbox:       "inbox",
		selectedMessages:      2,
		selectedHighestModSeq: 42,
		permanentFlags:        map[string]struct{}{FlagSeen: {}},
		savedSearch:           []imapSearchSavedMessage{{uid: 7, sequenceNumber: 1}},
	}

	done, err := server.handleClose(writer, "a1", state)
	if err != nil {
		t.Fatalf("handleClose returned error: %v", err)
	}
	if done {
		t.Fatal("handleClose done = true, want false")
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush close response: %v", err)
	}
	if output.String() != "a1 OK CLOSE completed\r\n" {
		t.Fatalf("close response = %q", output.String())
	}
	if backendImpl.expungeCount != 1 || backendImpl.expungeMailboxID != "inbox" || backendImpl.expungeUserID != "user-1" {
		t.Fatalf("close expunge = count %d user %q mailbox %q, want writable selected mailbox expunged", backendImpl.expungeCount, backendImpl.expungeUserID, backendImpl.expungeMailboxID)
	}
	if state.savedSearch != nil {
		t.Fatalf("savedSearch = %#v, want nil after CLOSE", state.savedSearch)
	}
	if state.selectedMailbox != "" || state.selectedMessages != 0 || state.selectedHighestModSeq != 0 || state.permanentFlags != nil || state.readOnly {
		t.Fatalf("selected state after CLOSE = mailbox %q messages %d modseq %d flags %#v readOnly %t", state.selectedMailbox, state.selectedMessages, state.selectedHighestModSeq, state.permanentFlags, state.readOnly)
	}
}

func TestServerCloseCondstoreAwareMailboxDrainsEventsAndResetsLifecycleState(t *testing.T) {
	t.Parallel()

	backendImpl := &closeBackend{}
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: backendImpl, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	events := make(chan MailboxEvent, 1)
	events <- MailboxEvent{Type: MailboxEventExists, UserID: "user-1", MailboxID: "inbox", Messages: 3}
	canceled := false
	var output strings.Builder
	writer := bufio.NewWriter(&output)
	state := &imapConnState{
		session:          &Session{UserID: "user-1"},
		selectedMailbox:  "inbox",
		selectedMessages: 2,
		selectedNoModSeq: true,
		condstoreAware:   true,
		permanentFlags:   map[string]struct{}{FlagSeen: {}},
		savedSearch:      []imapSearchSavedMessage{{uid: 7, sequenceNumber: 1}},
		events:           events,
		cancelEvents:     func() { canceled = true },
	}

	done, err := server.handleLineWithLiteral(writer, "a1 CLOSE\r\n", nil, state)
	if err != nil {
		t.Fatalf("handleLineWithLiteral returned error: %v", err)
	}
	if done {
		t.Fatal("handleLineWithLiteral done = true, want false")
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush close response: %v", err)
	}
	if got := output.String(); got != "* 3 EXISTS\r\na1 OK CLOSE completed\r\n" {
		t.Fatalf("close response = %q", got)
	}
	if strings.Contains(output.String(), "EXPUNGE") {
		t.Fatalf("CLOSE response unexpectedly emitted EXPUNGE: %q", output.String())
	}
	if backendImpl.expungeCount != 1 || backendImpl.expungeMailboxID != "inbox" || backendImpl.expungeUserID != "user-1" {
		t.Fatalf("close expunge = count %d user %q mailbox %q, want writable selected mailbox expunged", backendImpl.expungeCount, backendImpl.expungeUserID, backendImpl.expungeMailboxID)
	}
	if !canceled {
		t.Fatal("event subscription was not canceled")
	}
	if !state.condstoreAware {
		t.Fatal("session CONDSTORE awareness was cleared by CLOSE")
	}
	if state.selectedMailbox != "" || state.selectedMessages != 0 || state.selectedHighestModSeq != 0 || state.selectedNoModSeq || state.permanentFlags != nil || state.readOnly || state.savedSearch != nil || state.events != nil || state.cancelEvents != nil {
		t.Fatalf("selected state after CLOSE = mailbox %q messages %d modseq %d noModSeq %t flags %#v readOnly %t saved %#v events %#v cancel nil %t", state.selectedMailbox, state.selectedMessages, state.selectedHighestModSeq, state.selectedNoModSeq, state.permanentFlags, state.readOnly, state.savedSearch, state.events, state.cancelEvents == nil)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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

func TestServerHandlesExpunge(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 EXPUNGE\r\na4 UID EXPUNGE 7,999\r\n")); err != nil {
		t.Fatalf("write expunge: %v", err)
	}
	want := []string{
		"* 1 EXPUNGE\r\n",
		"a3 OK EXPUNGE completed\r\n",
		"* 1 EXPUNGE\r\n",
		"a4 OK UID EXPUNGE completed\r\n",
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

func TestServerRejectsUnsupportedMoveAndAppend(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 MOVE 1 Archive\r\na4 UID MOVE 7,999 Archive\r\na5 APPEND inbox NIL\r\n")); err != nil {
		t.Fatalf("write unsupported mutation commands: %v", err)
	}
	want := []string{
		"* OK [HIGHESTMODSEQ 19] MOVE source mod-sequence\r\n",
		"* OK [COPYUID 2 7 9] MOVE copied UIDs\r\n",
		"* 1 EXPUNGE\r\n",
		"a3 OK MOVE completed\r\n",
		"* OK [HIGHESTMODSEQ 19] UID MOVE source mod-sequence\r\n",
		"* OK [COPYUID 2 7 9] UID MOVE copied UIDs\r\n",
		"* 1 EXPUNGE\r\n",
		"a4 OK UID MOVE completed\r\n",
		"a5 BAD APPEND requires mailbox and literal\r\n",
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
	if _, err := client.Write([]byte("a6 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesMoveToQuotedMailboxName(t *testing.T) {
	t.Parallel()

	backendImpl := &quotedMailboxTransferBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID MOVE 7 \"Team Archive\"\r\n")); err != nil {
		t.Fatalf("write quoted move: %v", err)
	}
	want := []string{
		"* OK [HIGHESTMODSEQ 19] UID MOVE source mod-sequence\r\n",
		"* OK [COPYUID 7 7 30] UID MOVE copied UIDs\r\n",
		"* 1 EXPUNGE\r\n",
		"a3 OK UID MOVE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read quoted move response: %v", err)
		}
		if line != expected {
			t.Fatalf("quoted move response = %q, want %q", line, expected)
		}
	}
	if len(backendImpl.moveRequests) != 1 {
		t.Fatalf("move request count = %d, want 1", len(backendImpl.moveRequests))
	}
	if got, want := backendImpl.mailboxLookups, []MailboxID{"Team Archive"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mailbox lookups = %v, want %v", got, want)
	}
	if req := backendImpl.moveRequests[0]; req.UserID != "user-1" || req.SourceMailboxID != "inbox" || req.DestMailboxID != "team-archive" || !reflect.DeepEqual(req.UIDs, []UID{7}) {
		t.Fatalf("move request = %+v, want user-1 inbox -> team-archive UID 7", req)
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

func TestServerAllowsMoveToSelectedMailbox(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: sameMailboxMoveBackend{}, AllowInsecureAuth: true})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID MOVE 7 INBOX\r\n")); err != nil {
		t.Fatalf("write same-mailbox move: %v", err)
	}
	want := []string{
		"* 3 EXISTS\r\n",
		"* OK [HIGHESTMODSEQ 19] UID MOVE source mod-sequence\r\n",
		"* OK [COPYUID 1 7 9] UID MOVE copied UIDs\r\n",
		"* 1 EXPUNGE\r\n",
		"a3 OK UID MOVE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read same-mailbox move response: %v", err)
		}
		if line != expected {
			t.Fatalf("same-mailbox move response = %q, want %q", line, expected)
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

func TestServerConsumesAppendSynchronizingLiteralBeforeUnsupportedResponse(t *testing.T) {
	t.Parallel()

	backendImpl := &appendBackend{err: ErrUnsupportedAppend}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 APPEND inbox (\\Seen \\Flagged) \"05-May-2026 12:34:56 +0900\" {11}\r\n")); err != nil {
		t.Fatalf("write append literal command: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("append continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("hello world\r\n")); err != nil {
		t.Fatalf("write append literal body: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 NO APPEND is not supported\r\n" {
		t.Fatalf("append response = %q err = %v", line, err)
	}
	if backendImpl.request.UserID != "user-1" || backendImpl.request.MailboxID != "inbox" || backendImpl.body != "hello world" || backendImpl.request.Size != 11 {
		t.Fatalf("append request = user %q mailbox %q size %d body %q", backendImpl.request.UserID, backendImpl.request.MailboxID, backendImpl.request.Size, backendImpl.body)
	}
	if !backendImpl.request.Flags.Read || !backendImpl.request.Flags.Starred {
		t.Fatalf("append flags = %#v, want seen and flagged", backendImpl.request.Flags)
	}
	wantDate := time.Date(2026, 5, 5, 12, 34, 56, 0, time.FixedZone("", 9*60*60))
	if !backendImpl.request.InternalDate.Equal(wantDate) {
		t.Fatalf("append internal date = %s, want %s", backendImpl.request.InternalDate, wantDate)
	}
	if _, err := client.Write([]byte("a3 NOOP\r\n")); err != nil {
		t.Fatalf("write noop after append: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 OK NOOP completed\r\n" {
		t.Fatalf("noop after append = %q err = %v", line, err)
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

func TestServerAppendSuccessReturnsAppendUID(t *testing.T) {
	t.Parallel()

	backendImpl := &appendBackend{
		result: AppendMessageResult{
			Summary:     MessageSummary{ID: "message-42", MailboxID: "inbox", UID: 42},
			UIDValidity: 99,
		},
	}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 APPEND inbox {11}\r\n")); err != nil {
		t.Fatalf("write append literal command: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("append continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("hello world\r\n")); err != nil {
		t.Fatalf("write append literal body: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 OK [APPENDUID 99 42] APPEND completed\r\n" {
		t.Fatalf("append response = %q err = %v", line, err)
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

func TestServerAppendPassesCustomKeywordFlags(t *testing.T) {
	t.Parallel()

	backendImpl := &appendBackend{
		result: AppendMessageResult{
			Summary:     MessageSummary{ID: "message-42", MailboxID: "inbox", UID: 42},
			UIDValidity: 99,
		},
	}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 APPEND inbox (\\Seen $Project ClientTag) {11}\r\n")); err != nil {
		t.Fatalf("write append literal command: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("append continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("hello world\r\n")); err != nil {
		t.Fatalf("write append literal body: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 OK [APPENDUID 99 42] APPEND completed\r\n" {
		t.Fatalf("append response = %q err = %v", line, err)
	}
	if !backendImpl.request.Flags.Read || !reflect.DeepEqual(backendImpl.request.Flags.Keywords, []string{"$Project", "ClientTag"}) {
		t.Fatalf("append flags = %#v, want seen plus custom keywords", backendImpl.request.Flags)
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

func TestServerAppendOmitsAppendUIDForUIDNotStickyResult(t *testing.T) {
	t.Parallel()

	backendImpl := &appendBackend{
		result: AppendMessageResult{
			Summary:      MessageSummary{ID: "message-42", MailboxID: "inbox", UID: 42},
			UIDValidity:  99,
			UIDNotSticky: true,
		},
	}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 APPEND inbox {11}\r\n")); err != nil {
		t.Fatalf("write append literal command: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("append continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("hello world\r\n")); err != nil {
		t.Fatalf("write append literal body: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 OK APPEND completed\r\n" {
		t.Fatalf("append response = %q err = %v", line, err)
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

func TestServerAppendSelectedMailboxUsesReturnedSequenceForExists(t *testing.T) {
	t.Parallel()

	backendImpl := &appendBackend{
		result: AppendMessageResult{
			Summary:     MessageSummary{ID: "message-42", MailboxID: "inbox", UID: 42, SequenceNumber: 3},
			UIDValidity: 99,
		},
	}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 APPEND inbox {11}\r\n")); err != nil {
		t.Fatalf("write selected append literal command: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("selected append continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("hello world\r\n")); err != nil {
		t.Fatalf("write selected append body: %v", err)
	}
	want := []string{
		"* 3 EXISTS\r\n",
		"a3 OK [APPENDUID 99 42] APPEND completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read selected append response: %v", err)
		}
		if line != expected {
			t.Fatalf("selected append response = %q, want %q", line, expected)
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

func TestServerAppendDrainsMailboxEventsBeforeCompletion(t *testing.T) {
	t.Parallel()

	backendImpl := &appendEventBackend{
		appendBackend: appendBackend{
			result: AppendMessageResult{
				Summary:     MessageSummary{ID: "message-42", MailboxID: "inbox", UID: 42, SequenceNumber: 3},
				UIDValidity: 99,
			},
		},
		events: make(chan MailboxEvent, 2),
	}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	backendImpl.events <- MailboxEvent{Type: MailboxEventFlags, UserID: "user-1", MailboxID: "inbox", UID: 7}
	if _, err := client.Write([]byte("a3 APPEND inbox {11+}\r\nhello world\r\n")); err != nil {
		t.Fatalf("write append literal+ command: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged))\r\n",
		"* 3 EXISTS\r\n",
		"a3 OK [APPENDUID 99 42] APPEND completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read append event response: %v", err)
		}
		if line != expected {
			t.Fatalf("append event response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 NOOP\r\n")); err != nil {
		t.Fatalf("write noop after append: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a4 OK NOOP completed\r\n" {
		t.Fatalf("noop after append = %q err = %v", line, err)
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

func TestServerAppendToExaminedMailboxIsReadOnly(t *testing.T) {
	t.Parallel()

	backendImpl := &appendBackend{
		result: AppendMessageResult{
			Summary:     MessageSummary{ID: "message-44", MailboxID: "inbox", UID: 44},
			UIDValidity: 99,
		},
	}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 EXAMINE inbox\r\n")); err != nil {
		t.Fatalf("write login/examine: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read examine response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 APPEND INBOX {5+}\r\nhello\r\n")); err != nil {
		t.Fatalf("write append literal+ command: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 NO mailbox is read-only\r\n" {
		t.Fatalf("append read-only response = %q err = %v", line, err)
	}
	if backendImpl.body != "" || backendImpl.request.UserID != "" || backendImpl.request.MailboxID != "" {
		t.Fatalf("append backend was called despite read-only mailbox: request=%+v body=%q", backendImpl.request, backendImpl.body)
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

func TestServerAppendAcceptsLiteralPlusWithoutContinuation(t *testing.T) {
	t.Parallel()

	backendImpl := &appendBackend{
		result: AppendMessageResult{
			Summary:     MessageSummary{ID: "message-43", MailboxID: "inbox", UID: 43},
			UIDValidity: 99,
		},
	}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 APPEND inbox {11+}\r\nhello world\r\n")); err != nil {
		t.Fatalf("write append literal+ command: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 OK [APPENDUID 99 43] APPEND completed\r\n" {
		t.Fatalf("append literal+ response = %q err = %v", line, err)
	}
	if backendImpl.request.UserID != "user-1" || backendImpl.request.MailboxID != "inbox" || backendImpl.body != "hello world" || backendImpl.request.Size != 11 {
		t.Fatalf("append literal+ request = user %q mailbox %q size %d body %q", backendImpl.request.UserID, backendImpl.request.MailboxID, backendImpl.request.Size, backendImpl.body)
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

func TestServerAppendMissingMailboxReturnsTryCreate(t *testing.T) {
	t.Parallel()

	backendImpl := &appendBackend{err: ErrMailboxNotFound}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 APPEND missing {11}\r\n")); err != nil {
		t.Fatalf("write append literal command: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("append continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("hello world\r\n")); err != nil {
		t.Fatalf("write append literal body: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 NO [TRYCREATE] APPEND mailbox does not exist\r\n" {
		t.Fatalf("append response = %q err = %v", line, err)
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

func TestServerAppendOverQuotaReturnsOverQuota(t *testing.T) {
	t.Parallel()

	backendImpl := &appendBackend{err: ErrOverQuota}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 APPEND inbox {11}\r\n")); err != nil {
		t.Fatalf("write append literal command: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("append continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("hello world\r\n")); err != nil {
		t.Fatalf("write append literal body: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 NO [OVERQUOTA] APPEND would exceed quota\r\n" {
		t.Fatalf("append response = %q err = %v", line, err)
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

func TestServerHandlesCopyCommands(t *testing.T) {
	t.Parallel()

	backendImpl := &copyBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 COPY 1:2 Archive\r\na4 UID COPY 7 Archive\r\na5 UID COPY 7,999 Archive\r\n")); err != nil {
		t.Fatalf("write copy commands: %v", err)
	}
	want := []string{
		"a3 OK [COPYUID 2 7:8 9:10] COPY completed\r\n",
		"a4 OK [COPYUID 2 7 11] UID COPY completed\r\n",
		"a5 OK [COPYUID 2 7 12] UID COPY completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read copy response: %v", err)
		}
		if line != expected {
			t.Fatalf("copy response = %q, want %q", line, expected)
		}
	}
	if len(backendImpl.requests) != 3 {
		t.Fatalf("copy request count = %d, want 3", len(backendImpl.requests))
	}
	if got, want := backendImpl.requests[0].UIDs, []UID{7, 8}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sequence COPY UIDs = %v, want %v", got, want)
	}
	if got, want := backendImpl.requests[1].UIDs, []UID{7}; !reflect.DeepEqual(got, want) {
		t.Fatalf("UID COPY UIDs = %v, want %v", got, want)
	}
	if got, want := backendImpl.requests[2].UIDs, []UID{7}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sparse UID COPY UIDs = %v, want %v", got, want)
	}
	for _, req := range backendImpl.requests {
		if req.SourceMailboxID != "inbox" || req.DestMailboxID != "archive" || req.UserID != "user-1" {
			t.Fatalf("copy request = %+v, want user-1 inbox -> archive", req)
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

func TestServerHandlesCopyToQuotedMailboxName(t *testing.T) {
	t.Parallel()

	backendImpl := &quotedMailboxTransferBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 COPY 1 \"Team Archive\"\r\na4 UID COPY 7 \"Team Archive\"\r\n")); err != nil {
		t.Fatalf("write quoted copy: %v", err)
	}
	want := []string{
		"a3 OK [COPYUID 7 7 20] COPY completed\r\n",
		"a4 OK [COPYUID 7 7 21] UID COPY completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read quoted copy response: %v", err)
		}
		if line != expected {
			t.Fatalf("quoted copy response = %q, want %q", line, expected)
		}
	}
	if len(backendImpl.copyRequests) != 2 {
		t.Fatalf("copy request count = %d, want 2", len(backendImpl.copyRequests))
	}
	if got, want := backendImpl.mailboxLookups, []MailboxID{"Team Archive", "Team Archive"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mailbox lookups = %v, want %v", got, want)
	}
	for _, req := range backendImpl.copyRequests {
		if req.UserID != "user-1" || req.SourceMailboxID != "inbox" || req.DestMailboxID != "team-archive" || !reflect.DeepEqual(req.UIDs, []UID{7}) {
			t.Fatalf("copy request = %+v, want user-1 inbox -> team-archive UID 7", req)
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

func TestServerHandlesCopyMoveToEscapedQuotedMailboxName(t *testing.T) {
	t.Parallel()

	backendImpl := &quotedMailboxTransferBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 COPY 1 \"Team \\\"Archive\\\"\"\r\na4 UID MOVE 7 \"Team \\\"Archive\\\"\"\r\n")); err != nil {
		t.Fatalf("write escaped quoted mailbox commands: %v", err)
	}
	want := []string{
		"a3 OK [COPYUID 8 7 20] COPY completed\r\n",
		"* OK [HIGHESTMODSEQ 19] UID MOVE source mod-sequence\r\n",
		"* OK [COPYUID 8 7 30] UID MOVE copied UIDs\r\n",
		"* 1 EXPUNGE\r\n",
		"a4 OK UID MOVE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read escaped quoted mailbox response: %v", err)
		}
		if line != expected {
			t.Fatalf("escaped quoted mailbox response = %q, want %q", line, expected)
		}
	}
	if got, want := backendImpl.mailboxLookups, []MailboxID{`Team "Archive"`, `Team "Archive"`}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mailbox lookups = %v, want %v", got, want)
	}
	if len(backendImpl.copyRequests) != 1 || len(backendImpl.moveRequests) != 1 {
		t.Fatalf("request counts = copy %d move %d, want 1 each", len(backendImpl.copyRequests), len(backendImpl.moveRequests))
	}
	if req := backendImpl.copyRequests[0]; req.DestMailboxID != "team-quote-archive" || !reflect.DeepEqual(req.UIDs, []UID{7}) {
		t.Fatalf("copy request = %+v, want team-quote-archive UID 7", req)
	}
	if req := backendImpl.moveRequests[0]; req.DestMailboxID != "team-quote-archive" || !reflect.DeepEqual(req.UIDs, []UID{7}) {
		t.Fatalf("move request = %+v, want team-quote-archive UID 7", req)
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

func TestServerHandlesCopyMoveToLiteralMailboxName(t *testing.T) {
	t.Parallel()

	backendImpl := &quotedMailboxTransferBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 COPY 1 {12}\r\n")); err != nil {
		t.Fatalf("write copy literal marker: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("copy literal continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("Team Archive\r\n")); err != nil {
		t.Fatalf("write copy literal mailbox: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 OK [COPYUID 7 7 20] COPY completed\r\n" {
		t.Fatalf("literal copy response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 UID MOVE 7 {12+}\r\nTeam Archive\r\n")); err != nil {
		t.Fatalf("write uid move literal+ mailbox: %v", err)
	}
	want := []string{
		"* OK [HIGHESTMODSEQ 19] UID MOVE source mod-sequence\r\n",
		"* OK [COPYUID 7 7 30] UID MOVE copied UIDs\r\n",
		"* 1 EXPUNGE\r\n",
		"a4 OK UID MOVE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read literal mailbox response: %v", err)
		}
		if line != expected {
			t.Fatalf("literal mailbox response = %q, want %q", line, expected)
		}
	}
	if got, want := backendImpl.mailboxLookups, []MailboxID{"Team Archive", "Team Archive"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mailbox lookups = %v, want %v", got, want)
	}
	if len(backendImpl.copyRequests) != 1 || len(backendImpl.moveRequests) != 1 {
		t.Fatalf("request counts = copy %d move %d, want 1 each", len(backendImpl.copyRequests), len(backendImpl.moveRequests))
	}
	if req := backendImpl.copyRequests[0]; req.DestMailboxID != "team-archive" || !reflect.DeepEqual(req.UIDs, []UID{7}) {
		t.Fatalf("copy request = %+v, want team-archive UID 7", req)
	}
	if req := backendImpl.moveRequests[0]; req.DestMailboxID != "team-archive" || !reflect.DeepEqual(req.UIDs, []UID{7}) {
		t.Fatalf("move request = %+v, want team-archive UID 7", req)
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

func TestServerCopyToSelectedMailboxUsesReturnedSequenceForExists(t *testing.T) {
	t.Parallel()

	backendImpl := &selectedCopyBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 COPY 1 INBOX\r\n")); err != nil {
		t.Fatalf("write selected copy: %v", err)
	}
	want := []string{
		"* 5 EXISTS\r\n",
		"a3 OK [COPYUID 1 7 11] COPY completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read selected copy response: %v", err)
		}
		if line != expected {
			t.Fatalf("selected copy response = %q, want %q", line, expected)
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

func TestServerCopyAndMoveMissingDestinationReturnsTryCreate(t *testing.T) {
	t.Parallel()

	backendImpl := missingDestinationBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 COPY 1 Missing\r\na4 UID MOVE 7 Missing\r\n")); err != nil {
		t.Fatalf("write missing destination commands: %v", err)
	}
	want := []string{
		"a3 NO [TRYCREATE] COPY destination mailbox does not exist\r\n",
		"a4 NO [TRYCREATE] UID MOVE destination mailbox does not exist\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read missing destination response: %v", err)
		}
		if line != expected {
			t.Fatalf("missing destination response = %q, want %q", line, expected)
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

func TestServerOmitsCopyUIDForUIDNotStickyDestination(t *testing.T) {
	t.Parallel()

	backendImpl := uidNotStickyDestinationBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 COPY 1 Archive\r\na4 UID MOVE 7 Archive\r\n")); err != nil {
		t.Fatalf("write uidnotsticky destination commands: %v", err)
	}
	want := []string{
		"a3 OK COPY completed\r\n",
		"* OK [HIGHESTMODSEQ 19] UID MOVE source mod-sequence\r\n",
		"* 1 EXPUNGE\r\n",
		"a4 OK UID MOVE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read uidnotsticky destination response: %v", err)
		}
		if line != expected {
			t.Fatalf("uidnotsticky destination response = %q, want %q", line, expected)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	backendImpl.events <- MailboxEvent{Type: MailboxEventExists, UserID: "user-1", MailboxID: "inbox", Messages: 2}
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

func TestServerDrainsExistsBeforeSequenceSetCommand(t *testing.T) {
	t.Parallel()

	backendImpl := &eventSequenceBackend{eventBackend: eventBackend{events: make(chan MailboxEvent, 4)}}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	backendImpl.events <- MailboxEvent{Type: MailboxEventExists, UserID: "user-1", MailboxID: "inbox", Messages: 3}
	if _, err := client.Write([]byte("a3 FETCH * (FLAGS)\r\n")); err != nil {
		t.Fatalf("write fetch: %v", err)
	}
	want := []string{
		"* 3 EXISTS\r\n",
		"* 3 FETCH (UID 9 FLAGS (\\Answered) RFC822.SIZE 9)\r\n",
		"a3 OK FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read fetch event response: %v", err)
		}
		if line != expected {
			t.Fatalf("fetch event response = %q, want %q", line, expected)
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

func TestServerDrainsExistsBeforeUIDSequenceSetCommand(t *testing.T) {
	t.Parallel()

	backendImpl := &eventSequenceBackend{eventBackend: eventBackend{events: make(chan MailboxEvent, 4)}}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	backendImpl.events <- MailboxEvent{Type: MailboxEventExists, UserID: "user-1", MailboxID: "inbox", Messages: 3}
	if _, err := client.Write([]byte("a3 UID FETCH * (FLAGS)\r\n")); err != nil {
		t.Fatalf("write uid fetch: %v", err)
	}
	want := []string{
		"* 3 EXISTS\r\n",
		"* 3 FETCH (UID 9 FLAGS (\\Answered) RFC822.SIZE 9)\r\n",
		"a3 OK UID FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read uid fetch event response: %v", err)
		}
		if line != expected {
			t.Fatalf("uid fetch event response = %q, want %q", line, expected)
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

func TestServerNoopIncludesModSeqForCondstoreAwareSession(t *testing.T) {
	t.Parallel()

	backendImpl := &eventBackend{events: make(chan MailboxEvent, 2)}
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
	for i := 0; i < 8; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read login/select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7 (MODSEQ)\r\n")); err != nil {
		t.Fatalf("write uid fetch modseq: %v", err)
	}
	for _, expected := range []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 MODSEQ (17))\r\n",
		"a3 OK UID FETCH completed\r\n",
	} {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read uid fetch modseq response: %v", err)
		}
		if line != expected {
			t.Fatalf("uid fetch modseq response = %q, want %q", line, expected)
		}
	}
	backendImpl.events <- MailboxEvent{Type: MailboxEventFlags, UserID: "user-1", MailboxID: "inbox", UID: 7}
	if _, err := client.Write([]byte("a4 NOOP\r\n")); err != nil {
		t.Fatalf("write noop: %v", err)
	}
	for _, expected := range []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) MODSEQ (17))\r\n",
		"a4 OK NOOP completed\r\n",
	} {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read noop response: %v", err)
		}
		if line != expected {
			t.Fatalf("noop response = %q, want %q", line, expected)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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

func TestServerRejectsUnexpectedCommandDuringIdleAndContinuesSession(t *testing.T) {
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
	for i := 0; i < 8; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read login/select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 IDLE\r\n")); err != nil {
		t.Fatalf("write idle: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ idling\r\n" {
		t.Fatalf("idle continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("NOOP\r\n")); err != nil {
		t.Fatalf("write unexpected idle noop: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 BAD IDLE terminated by unexpected command\r\n" {
		t.Fatalf("unexpected idle command response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 NOOP\r\n")); err != nil {
		t.Fatalf("write noop after idle bad: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a4 OK NOOP completed\r\n" {
		t.Fatalf("noop after idle bad = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a5 IDLE\r\n")); err != nil {
		t.Fatalf("write second idle: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ idling\r\n" {
		t.Fatalf("second idle continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("DONE NOW\r\n")); err != nil {
		t.Fatalf("write malformed done: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a5 BAD IDLE terminated by unexpected command\r\n" {
		t.Fatalf("malformed done response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a6 NOOP\r\n")); err != nil {
		t.Fatalf("write noop after malformed done: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a6 OK NOOP completed\r\n" {
		t.Fatalf("noop after malformed done = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a7 IDLE\r\n")); err != nil {
		t.Fatalf("write third idle: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ idling\r\n" {
		t.Fatalf("third idle continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte(" DONE\r\n")); err != nil {
		t.Fatalf("write space-padded done: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a7 BAD IDLE terminated by unexpected command\r\n" {
		t.Fatalf("space-padded done response = %q err = %v", line, err)
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

func TestServerReportsOversizedIdleLineBeforeClosing(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\na3 IDLE\r\n")); err != nil {
		t.Fatalf("write login/select/idle: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ idling\r\n" {
		t.Fatalf("idle continuation = %q err = %v", line, err)
	}
	writeErrCh := make(chan error, 1)
	go func() {
		_, err := client.Write([]byte(strings.Repeat("A", maxIMAPCommandLineBytes+1) + "\r\n"))
		writeErrCh <- err
	}()
	want := []string{
		"a3 BAD command line is too long\r\n",
		"* BYE gogomail IMAP4rev1 server closing connection after framing error\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read oversized idle response: %v", err)
		}
		if line != expected {
			t.Fatalf("oversized idle response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
	<-writeErrCh
}

func TestServerRejectsLFOnlyIdleDoneBeforeClosing(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\na3 IDLE\r\n")); err != nil {
		t.Fatalf("write login/select/idle: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ idling\r\n" {
		t.Fatalf("idle continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("DONE\n")); err != nil {
		t.Fatalf("write lf-only done: %v", err)
	}
	want := []string{
		"a3 BAD command line must end with CRLF\r\n",
		"* BYE gogomail IMAP4rev1 server closing connection after framing error\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read lf-only idle response: %v", err)
		}
		if line != expected {
			t.Fatalf("lf-only idle response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerDrainsExpungeEventsOverNoop(t *testing.T) {
	t.Parallel()

	backendImpl := &eventBackend{events: make(chan MailboxEvent, 2)}
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
	for i := 0; i < 8; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read login/select response: %v", err)
		}
	}
	backendImpl.events <- MailboxEvent{Type: MailboxEventExpunge, UserID: "user-1", MailboxID: "inbox", UID: 8, SequenceNumber: 2}
	if _, err := client.Write([]byte("a3 NOOP\r\n")); err != nil {
		t.Fatalf("write noop: %v", err)
	}
	want := []string{
		"* 2 EXPUNGE\r\n",
		"a3 OK NOOP completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read noop expunge response: %v", err)
		}
		if line != expected {
			t.Fatalf("noop expunge response = %q, want %q", line, expected)
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

func TestMailboxExpungeEventUpdatesSavedSearchSequences(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	writer := bufio.NewWriter(&out)
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	state := &imapConnState{
		session:          &Session{UserID: "user-1"},
		selectedMailbox:  "inbox",
		selectedMessages: 3,
		savedSearch: []imapSearchSavedMessage{
			{uid: 7, sequenceNumber: 1},
			{uid: 8, sequenceNumber: 2},
			{uid: 9, sequenceNumber: 3},
		},
	}
	err = server.writeMailboxEvent(writer, state, MailboxEvent{
		Type:           MailboxEventExpunge,
		UserID:         "user-1",
		MailboxID:      "inbox",
		UID:            7,
		SequenceNumber: 1,
	})
	if err != nil {
		t.Fatalf("writeMailboxEvent returned error: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush event: %v", err)
	}
	if got, want := out.String(), "* 1 EXPUNGE\r\n"; got != want {
		t.Fatalf("expunge event output = %q, want %q", got, want)
	}
	if state.selectedMessages != 2 {
		t.Fatalf("selectedMessages = %d, want 2", state.selectedMessages)
	}
	wantSaved := []imapSearchSavedMessage{
		{uid: 8, sequenceNumber: 1},
		{uid: 9, sequenceNumber: 2},
	}
	if !reflect.DeepEqual(state.savedSearch, wantSaved) {
		t.Fatalf("saved search = %#v, want %#v", state.savedSearch, wantSaved)
	}
}

func TestMovedExpungeResponsesUpdateSavedSearchForMultipleExpunges(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	writer := bufio.NewWriter(&out)
	state := &imapConnState{
		selectedMailbox:  "inbox",
		selectedMessages: 4,
		savedSearch: []imapSearchSavedMessage{
			{uid: 7, sequenceNumber: 1},
			{uid: 8, sequenceNumber: 2},
			{uid: 9, sequenceNumber: 3},
			{uid: 10, sequenceNumber: 4},
		},
	}
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	if _, err := server.writeMovedExpungeResponses(writer, "a1", state, []MessageSummary{
		{UID: 7, SequenceNumber: 1},
		{UID: 9, SequenceNumber: 3},
	}, "UID EXPUNGE", ""); err != nil {
		t.Fatalf("writeMovedExpungeResponses returned error: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush expunge responses: %v", err)
	}
	if got, want := out.String(), "* 1 EXPUNGE\r\n* 2 EXPUNGE\r\na1 OK UID EXPUNGE completed\r\n"; got != want {
		t.Fatalf("expunge response output = %q, want %q", got, want)
	}
	wantSaved := []imapSearchSavedMessage{
		{uid: 8, sequenceNumber: 1},
		{uid: 10, sequenceNumber: 2},
	}
	if !reflect.DeepEqual(state.savedSearch, wantSaved) {
		t.Fatalf("saved search = %#v, want %#v", state.savedSearch, wantSaved)
	}
	if state.selectedMessages != 2 {
		t.Fatalf("selectedMessages = %d, want 2", state.selectedMessages)
	}
}

func TestServerStreamsExpungeEventsOverIdle(t *testing.T) {
	t.Parallel()

	backendImpl := &eventBackend{events: make(chan MailboxEvent, 2)}
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
	for i := 0; i < 8; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read login/select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 IDLE\r\n")); err != nil {
		t.Fatalf("write idle: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ idling\r\n" {
		t.Fatalf("idle continuation = %q err = %v", line, err)
	}
	backendImpl.events <- MailboxEvent{Type: MailboxEventExpunge, UserID: "user-1", MailboxID: "inbox", UID: 7, SequenceNumber: 1}
	if err := client.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* 1 EXPUNGE\r\n" {
		t.Fatalf("idle expunge event = %q err = %v", line, err)
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" * RETURN (CHILDREN)\r\n")); err != nil {
		t.Fatalf("write login/list: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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

func TestServerListUsesModifiedUTF7MailboxNames(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: unicodeMailboxBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" \"~peter/mail/&U,BTFw-/*\"\r\n")); err != nil {
		t.Fatalf("write unicode list: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\HasNoChildren) \"/\" \"~peter/mail/&U,BTFw-/&ZeVnLIqe-\"\r\n",
		"a2 OK LIST completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read unicode list response: %v", err)
		}
		if line != expected {
			t.Fatalf("unicode list response = %q, want %q", line, expected)
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

func TestServerListPreservesMailboxNameSpacing(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: spacedMailboxBackend{}, AllowInsecureAuth: true})
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
		t.Fatalf("write spaced list: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\HasNoChildren) \"/\" \"Project  Q2\"\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"Archive 2026\"\r\n",
		"a2 OK LIST completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read spaced list response: %v", err)
		}
		if line != expected {
			t.Fatalf("spaced list response = %q, want %q", line, expected)
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

func TestServerListReportsMailboxChildren(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: childMailboxBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" *\r\na3 LIST \"/Projects\" \"2026\"\r\n")); err != nil {
		t.Fatalf("write login/list: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"* LIST (\\HasChildren) \"/\" \"Projects\"\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"Projects/2026\"\r\n",
		"a2 OK LIST completed\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"Projects/2026\"\r\n",
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

func TestServerListInfersNestedMailboxChildrenFromFullPath(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: nestedMailboxBackend{}, AllowInsecureAuth: true})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"* LIST (\\HasChildren) \"/\" \"Projects\"\r\n",
		"* LIST (\\HasChildren) \"/\" \"Projects/2026\"\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"Projects/2026/Jan\"\r\n",
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

func TestServerListReportsSpecialUseAttributes(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: specialUseBackend{}, AllowInsecureAuth: true})
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
		t.Fatalf("write login/list special-use: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"* LIST (\\HasNoChildren \\Drafts) \"/\" \"Drafts\"\r\n",
		"* LIST (\\HasNoChildren \\Sent) \"/\" \"Sent\"\r\n",
		"* LIST (\\HasNoChildren \\Trash) \"/\" \"Trash\"\r\n",
		"a2 OK LIST completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read special-use list response: %v", err)
		}
		if line != expected {
			t.Fatalf("special-use list response = %q, want %q", line, expected)
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

func TestServerListSupportsSpecialUseSelectionAndReturn(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: specialUseBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST (SPECIAL-USE) \"\" *\r\na3 LIST \"\" * RETURN (SPECIAL-USE)\r\na4 LIST (REMOTE) \"\" *\r\na5 LIST \" (SPECIAL-USE) \" \"\" *\r\na6 LIST (SPECIAL-USE ) \"\" *\r\n")); err != nil {
		t.Fatalf("write login/list special-use extended: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\HasNoChildren \\Drafts) \"/\" \"Drafts\"\r\n",
		"* LIST (\\HasNoChildren \\Sent) \"/\" \"Sent\"\r\n",
		"* LIST (\\HasNoChildren \\Trash) \"/\" \"Trash\"\r\n",
		"a2 OK LIST completed\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"* LIST (\\HasNoChildren \\Drafts) \"/\" \"Drafts\"\r\n",
		"* LIST (\\HasNoChildren \\Sent) \"/\" \"Sent\"\r\n",
		"* LIST (\\HasNoChildren \\Trash) \"/\" \"Trash\"\r\n",
		"a3 OK LIST completed\r\n",
		"a4 BAD LIST requires reference and mailbox pattern atoms\r\n",
		"a5 BAD LIST requires reference and mailbox pattern atoms\r\n",
		"a6 BAD LIST requires reference and mailbox pattern atoms\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read extended special-use list response: %v", err)
		}
		if line != expected {
			t.Fatalf("extended special-use list response = %q, want %q", line, expected)
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

func TestServerRejectsStringListControlListsBeforeState(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LIST \"(SPECIAL-USE)\" \"\" *\r\na2 LIST {13+}\r\n(SPECIAL-USE) \"\" *\r\na3 LIST \"\" * RETURN \"(SPECIAL-USE)\"\r\na4 LIST \"\" * RETURN {13+}\r\n(SPECIAL-USE)\r\na5 LIST \"\" * RETURN \"(STATUS (MESSAGES))\"\r\na6 LIST \"\" * \"RETURN\" (SPECIAL-USE)\r\na7 LIST \"\" * {6+}\r\nRETURN (SPECIAL-USE)\r\na8 LIST \"\" (\"INBOX\" \"Sent\")\r\na9 LOGOUT\r\n")); err != nil {
		t.Fatalf("write string list control lists: %v", err)
	}
	want := []string{
		"a1 BAD LIST requires reference and mailbox pattern atoms\r\n",
		"a2 BAD LIST requires reference and mailbox pattern atoms\r\n",
		"a3 BAD LIST requires parenthesized return options\r\n",
		"a4 BAD LIST requires parenthesized return options\r\n",
		"a5 BAD LIST requires parenthesized return options\r\n",
		"a6 BAD LIST requires return options atom\r\n",
		"a7 BAD LIST requires return options atom\r\n",
		"a8 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a9 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read string list control-list response: %v", err)
		}
		if line != expected {
			t.Fatalf("string list control-list response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerListSupportsStatusReturnOption(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: listStatusBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" * RETURN (STATUS (MESSAGES UNSEEN UIDNEXT HIGHESTMODSEQ SIZE))\r\na3 LIST \"\" * RETURN (SPECIAL-USE STATUS (MESSAGES SIZE))\r\na4 LIST \"\" * RETURN (STATUS)\r\na5 LIST \"\" * RETURN (STATUS MESSAGES)\r\na6 LIST \"\" * RETURN (STATUS (MESSAGES MESSAGES))\r\na7 LIST \"\" * RETURN (CHILDREN STATUS (MESSAGES))\r\na8 LIST \"\" * RETURN (STATUS ())\r\na9 LIST \"\" * RETURN (STATUS ( ))\r\na10 LIST \"\" * RETURN (STATUS (MESSAGES) CHILDREN STATUS (UNSEEN))\r\na11 LIST \"\" * RETURN CHILDREN\r\na12 LIST \"\" * RETURN \" (CHILDREN) \"\r\na13 LIST \"\" * RETURN \" (STATUS (MESSAGES)) \"\r\n")); err != nil {
		t.Fatalf("write list-status: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"* STATUS \"INBOX\" (MESSAGES 17 UNSEEN 3 UIDNEXT 41 HIGHESTMODSEQ 70 SIZE 4096)\r\n",
		"* LIST (\\HasNoChildren \\Sent) \"/\" \"Sent\"\r\n",
		"* STATUS \"Sent\" (MESSAGES 5 UNSEEN 0 UIDNEXT 8 HIGHESTMODSEQ 12 SIZE 2048)\r\n",
		"a2 OK LIST completed\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"* STATUS \"INBOX\" (MESSAGES 17 SIZE 4096)\r\n",
		"* LIST (\\HasNoChildren \\Sent) \"/\" \"Sent\"\r\n",
		"* STATUS \"Sent\" (MESSAGES 5 SIZE 2048)\r\n",
		"a3 OK LIST completed\r\n",
		"a4 BAD LIST requires parenthesized status item list\r\n",
		"a5 BAD LIST requires parenthesized status item list\r\n",
		"a6 BAD LIST status item is duplicated\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"* STATUS \"INBOX\" (MESSAGES 17)\r\n",
		"* LIST (\\HasNoChildren \\Sent) \"/\" \"Sent\"\r\n",
		"* STATUS \"Sent\" (MESSAGES 5)\r\n",
		"a7 OK LIST completed\r\n",
		"a8 BAD LIST requires status data items\r\n",
		"a9 BAD LIST requires status data items\r\n",
		"a10 BAD LIST status return option is duplicated\r\n",
		"a11 BAD LIST requires parenthesized return options\r\n",
		"a12 BAD LIST requires parenthesized return options\r\n",
		"a13 BAD LIST requires parenthesized return options\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read list-status response: %v", err)
		}
		if line != expected {
			t.Fatalf("list-status response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a14 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerListSupportsExtendedPatternListsWithStatus(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: listStatusBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" (\"INBOX\" \"Sent\") RETURN (STATUS (MESSAGES))\r\na3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write list pattern-list: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"* STATUS \"INBOX\" (MESSAGES 17)\r\n",
		"* LIST (\\HasNoChildren \\Sent) \"/\" \"Sent\"\r\n",
		"* STATUS \"Sent\" (MESSAGES 5)\r\n",
		"a2 OK LIST completed\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a3 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read list pattern-list response: %v", err)
		}
		if line != expected {
			t.Fatalf("list pattern-list response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerListSupportsQuotedPatternListsWithSpaces(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: spacedListPatternBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" (\"Archive 2026\" \"INBOX\") RETURN (STATUS (MESSAGES))\r\na3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write spaced list pattern-list: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"* STATUS \"INBOX\" (MESSAGES 17)\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"Archive 2026\"\r\n",
		"* STATUS \"Archive 2026\" (MESSAGES 9)\r\n",
		"a2 OK LIST completed\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a3 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read spaced list pattern-list response: %v", err)
		}
		if line != expected {
			t.Fatalf("spaced list pattern-list response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerListSupportsLiteralPatternListsWithSpaces(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: spacedListPatternBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" ({12}\r\n")); err != nil {
		t.Fatalf("write literal list pattern-list command: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login response = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("literal continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("Archive 2026 \"INBOX\") RETURN (STATUS (MESSAGES))\r\na3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write literal list pattern-list suffix: %v", err)
	}
	want := []string{
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"* STATUS \"INBOX\" (MESSAGES 17)\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"Archive 2026\"\r\n",
		"* STATUS \"Archive 2026\" (MESSAGES 9)\r\n",
		"a2 OK LIST completed\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a3 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read literal list pattern-list response: %v", err)
		}
		if line != expected {
			t.Fatalf("literal list pattern-list response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerListSupportsSubscribedExtendedPatternLists(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: subscriptionBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST (SUBSCRIBED) \"\" (\"INBOX\" \"Retired\") RETURN (SUBSCRIBED)\r\na3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write subscribed list pattern-list: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* LIST (\\HasNoChildren \\Subscribed) \"/\" \"INBOX\"\r\n",
		"* LIST (\\Noselect \\Subscribed) \"/\" \"Retired\"\r\n",
		"a2 OK LIST completed\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a3 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read subscribed list pattern-list response: %v", err)
		}
		if line != expected {
			t.Fatalf("subscribed list pattern-list response = %q, want %q", line, expected)
		}
	}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" \"INBOX\"\r\na3 LIST \"\" \"Archive%\"\r\na4 LIST \"Archive\" \"/INBOX\"\r\n")); err != nil {
		t.Fatalf("write login/list: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"a2 OK LIST completed\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"Archive 2026\"\r\n",
		"a3 OK LIST completed\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"a4 OK LIST completed\r\n",
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
	if _, err := client.Write([]byte("a5 LOGOUT\r\n")); err != nil {
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

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: subscriptionBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LSUB \"\" \"INBOX\"\r\na3 LSUB \"Archive\" \"/INBOX\"\r\n")); err != nil {
		t.Fatalf("write login/lsub: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LSUB (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"a2 OK LSUB completed\r\n",
		"* LSUB (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"a3 OK LSUB completed\r\n",
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
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerLsubIncludesMissingSubscriptionNames(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: subscriptionBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LSUB \"\" \"*\"\r\n")); err != nil {
		t.Fatalf("write login/lsub: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LSUB (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"* LSUB (\\Noselect) \"/\" \"Retired\"\r\n",
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

func TestServerListSupportsSubscribedSelectionAndReturn(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: subscriptionBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST (SUBSCRIBED) \"\" \"*\" RETURN (SUBSCRIBED CHILDREN)\r\n")); err != nil {
		t.Fatalf("write login/list subscribed: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\HasNoChildren \\Subscribed) \"/\" \"INBOX\"\r\n",
		"* LIST (\\Noselect \\Subscribed) \"/\" \"Retired\"\r\n",
		"a2 OK LIST completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read list subscribed response: %v", err)
		}
		if line != expected {
			t.Fatalf("list subscribed response = %q, want %q", line, expected)
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

func TestServerListSupportsSubscribedReturnOption(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" \"*\" RETURN (SUBSCRIBED)\r\n")); err != nil {
		t.Fatalf("write login/list subscribed return: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\HasNoChildren \\Subscribed) \"/\" \"INBOX\"\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"Archive 2026\"\r\n",
		"a2 OK LIST completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read list subscribed return response: %v", err)
		}
		if line != expected {
			t.Fatalf("list subscribed return response = %q, want %q", line, expected)
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

func TestServerLsubPercentReturnsSubscribedParentName(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: hierarchySubscriptionBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LSUB \"\" \"%\"\r\n")); err != nil {
		t.Fatalf("write login/lsub: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LSUB (\\Noselect) \"/\" \"Projects\"\r\n",
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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

	backendImpl := &subscriptionCommandBackend{}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SUBSCRIBE inbox\r\na3 UNSUBSCRIBE inbox\r\n")); err != nil {
		t.Fatalf("write subscription commands: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
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
	if backendImpl.subscribed != "inbox" || backendImpl.unsubscribed != "inbox" {
		t.Fatalf("subscription calls = %q/%q, want inbox/inbox", backendImpl.subscribed, backendImpl.unsubscribed)
	}
}

func TestServerDecodesModifiedUTF7MailboxMutationArguments(t *testing.T) {
	t.Parallel()

	backendImpl := &mailboxMutationBackend{}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 CREATE &ZeVnLIqe-\r\na3 RENAME &ZeVnLIqe- &U,BTFw-\r\na4 SUBSCRIBE &U,BTFw-\r\na5 UNSUBSCRIBE &U,BTFw-\r\n")); err != nil {
		t.Fatalf("write mailbox commands: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a2 OK CREATE completed\r\n",
		"a3 OK RENAME completed\r\n",
		"a4 OK SUBSCRIBE completed\r\n",
		"a5 OK UNSUBSCRIBE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read mailbox command response: %v", err)
		}
		if line != expected {
			t.Fatalf("mailbox command response = %q, want %q", line, expected)
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
	if backendImpl.created != "日本語" || backendImpl.renamedFrom != "日本語" || backendImpl.renamedTo != "台北" || backendImpl.subscribed != "台北" || backendImpl.unsubscribed != "台北" {
		t.Fatalf("decoded mailbox args = create %q rename %q/%q subscribe %q unsubscribe %q", backendImpl.created, backendImpl.renamedFrom, backendImpl.renamedTo, backendImpl.subscribed, backendImpl.unsubscribed)
	}
}

func TestServerPreservesPaddedINBOXMailboxMutationArguments(t *testing.T) {
	t.Parallel()

	backendImpl := &mailboxMutationBackend{}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 CREATE \" INBOX \"\r\na3 RENAME \" INBOX \" \" inbox \"\r\n")); err != nil {
		t.Fatalf("write padded inbox mailbox commands: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a2 OK CREATE completed\r\n",
		"a3 OK RENAME completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read padded inbox mailbox response: %v", err)
		}
		if line != expected {
			t.Fatalf("padded inbox mailbox response = %q, want %q", line, expected)
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
	if backendImpl.created != " INBOX " || backendImpl.renamedFrom != " INBOX " || backendImpl.renamedTo != " inbox " {
		t.Fatalf("padded inbox mailbox args = create %q rename %q/%q", backendImpl.created, backendImpl.renamedFrom, backendImpl.renamedTo)
	}
}

func TestServerDecodesModifiedUTF7OperationalMailboxArguments(t *testing.T) {
	t.Parallel()

	backendImpl := &operationalMailboxNameBackend{}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a2 SELECT &U,BTFw-\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 6; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select metadata: %v", err)
		}
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 OK [READ-WRITE] SELECT completed\r\n" {
		t.Fatalf("select completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a3 STATUS &U,BTFw- (MESSAGES UIDNEXT)\r\n")); err != nil {
		t.Fatalf("write status: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* STATUS \"&U,BTFw-\" (MESSAGES 2 UIDNEXT 12)\r\n" {
		t.Fatalf("status line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 OK STATUS completed\r\n" {
		t.Fatalf("status completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 APPEND &U,BTFw- {5}\r\n")); err != nil {
		t.Fatalf("write append: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "+ Ready for literal data\r\n" {
		t.Fatalf("append continuation = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("hello\r\n")); err != nil {
		t.Fatalf("write append body: %v", err)
	}
	wantAppend := []string{
		"* 3 EXISTS\r\n",
		"a4 OK [APPENDUID 10 44] APPEND completed\r\n",
	}
	for _, expected := range wantAppend {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read append response: %v", err)
		}
		if line != expected {
			t.Fatalf("append response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a5 COPY 1 &ZeVnLIqe-\r\na6 UID MOVE 7 &ZeVnLIqe-\r\na7 RENAME &ZeVnLIqe- &U,BTFw-\r\n")); err != nil {
		t.Fatalf("write copy/move: %v", err)
	}
	want := []string{
		"a5 OK [COPYUID 20 7 50] COPY completed\r\n",
		"* OK [HIGHESTMODSEQ 30] UID MOVE source mod-sequence\r\n",
		"* OK [COPYUID 20 7 51] UID MOVE copied UIDs\r\n",
		"* 1 EXPUNGE\r\n",
		"a6 OK UID MOVE completed\r\n",
		"a7 OK RENAME completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read copy/move response: %v", err)
		}
		if line != expected {
			t.Fatalf("copy/move response = %q, want %q", line, expected)
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
	if backendImpl.selected != "台北" || backendImpl.statusLookup != "台北" || backendImpl.appended != "taipei" || backendImpl.appendBody != "hello" {
		t.Fatalf("decoded select/status/append = %q/%q/%q body %q", backendImpl.selected, backendImpl.statusLookup, backendImpl.appended, backendImpl.appendBody)
	}
	if backendImpl.copyDest != "nihon" || backendImpl.moveDest != "nihon" {
		t.Fatalf("decoded copy/move destination IDs = %q/%q, want nihon/nihon", backendImpl.copyDest, backendImpl.moveDest)
	}
	if backendImpl.renameSource != "nihon" || backendImpl.renameDest != "台北" {
		t.Fatalf("decoded rename source/dest = %q/%q, want nihon/台北", backendImpl.renameSource, backendImpl.renameDest)
	}
}

func TestServerRejectsMalformedModifiedUTF7MailboxName(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 CREATE &Jjo!\r\n")); err != nil {
		t.Fatalf("write bad mailbox command: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a2 BAD CREATE mailbox name is not valid modified UTF-7\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read bad mailbox response: %v", err)
		}
		if line != expected {
			t.Fatalf("bad mailbox response = %q, want %q", line, expected)
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

func TestServerValidatesMailboxCommandSyntaxBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LIST \"\"\r\na2 LIST \"\" &Jjo!\r\na3 LSUB \"\"\r\na4 CREATE &Jjo!\r\na5 DELETE &Jjo!\r\na6 RENAME Archive\r\na7 RENAME Archive &Jjo!\r\na8 SUBSCRIBE\r\na9 SUBSCRIBE &Jjo!\r\na10 CREATE Projects\r\na11 LIST \"\" INBOX\"\r\na12 LSUB \"\" INBOX\"\r\na13 LSUB (SPECIAL-USE) \"\" *\r\na14 LSUB \"\" * RETURN (STATUS (MESSAGES))\r\na15 LIST \"\" ()\r\na16 LOGOUT\r\n")); err != nil {
		t.Fatalf("write mailbox commands: %v", err)
	}
	want := []string{
		"a1 BAD LIST requires reference and mailbox pattern atoms\r\n",
		"a2 BAD LIST mailbox pattern is not valid modified UTF-7\r\n",
		"a3 BAD LSUB requires reference and mailbox pattern atoms\r\n",
		"a4 BAD CREATE mailbox name is not valid modified UTF-7\r\n",
		"a5 BAD DELETE mailbox name is not valid modified UTF-7\r\n",
		"a6 BAD RENAME requires source and destination mailbox names\r\n",
		"a7 BAD RENAME mailbox name is not valid modified UTF-7\r\n",
		"a8 BAD SUBSCRIBE requires a mailbox atom\r\n",
		"a9 BAD SUBSCRIBE mailbox name is not valid modified UTF-7\r\n",
		"a10 NO authentication required\r\n",
		"a11 BAD malformed command\r\n",
		"a12 BAD malformed command\r\n",
		"a13 BAD LSUB does not support LIST extension options\r\n",
		"a14 BAD LSUB does not support LIST extension options\r\n",
		"a15 BAD LIST mailbox pattern is not valid modified UTF-7\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a16 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read mailbox syntax response: %v", err)
		}
		if line != expected {
			t.Fatalf("mailbox syntax response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerValidatesSelectedMailboxSyntaxBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 NAMESPACE extra\r\na2 SELECT\r\na3 SELECT &Jjo!\r\na4 EXAMINE inbox (QRESYNC)\r\na5 STATUS\r\na6 STATUS inbox MESSAGES\r\na7 STATUS inbox (BADITEM)\r\na8 STATUS &Jjo! (MESSAGES)\r\na9 SELECT inbox CONDSTORE\r\na10 EXAMINE inbox ((CONDSTORE))\r\na11 SELECT inbox \" (CONDSTORE) \"\r\na12 EXAMINE inbox \" (CONDSTORE) \"\r\na13 SELECT inbox\r\na14 STATUS inbox (MESSAGES)\r\na15 STATUS inbox (MESSAGES MESSAGES)\r\na16 NAMESPACE\r\na17 LOGOUT\r\n")); err != nil {
		t.Fatalf("write selected mailbox syntax commands: %v", err)
	}
	want := []string{
		"a1 BAD NAMESPACE does not accept arguments\r\n",
		"a2 BAD SELECT requires a mailbox atom and optional CONDSTORE parameter\r\n",
		"a3 BAD SELECT mailbox name is not valid modified UTF-7\r\n",
		"a4 BAD EXAMINE requires a mailbox atom and optional CONDSTORE parameter\r\n",
		"a5 BAD STATUS requires mailbox and status item atoms\r\n",
		"a6 BAD STATUS requires parenthesized item list\r\n",
		"a7 BAD STATUS item is unsupported\r\n",
		"a8 BAD STATUS mailbox name is not valid modified UTF-7\r\n",
		"a9 BAD SELECT requires a mailbox atom and optional CONDSTORE parameter\r\n",
		"a10 BAD EXAMINE requires a mailbox atom and optional CONDSTORE parameter\r\n",
		"a11 BAD SELECT requires a mailbox atom and optional CONDSTORE parameter\r\n",
		"a12 BAD EXAMINE requires a mailbox atom and optional CONDSTORE parameter\r\n",
		"a13 NO authentication required\r\n",
		"a14 NO authentication required\r\n",
		"a15 BAD STATUS item is duplicated\r\n",
		"a16 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a17 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read selected mailbox syntax response: %v", err)
		}
		if line != expected {
			t.Fatalf("selected mailbox syntax response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsSubscriptionCommandsBeforeLogin(t *testing.T) {
	t.Parallel()

	for _, command := range []string{
		`LSUB "" "*"`,
		`SUBSCRIBE inbox`,
		`UNSUBSCRIBE inbox`,
	} {
		command := command
		t.Run(command, func(t *testing.T) {
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
			if _, err := client.Write([]byte("a1 " + command + "\r\na2 LOGOUT\r\n")); err != nil {
				t.Fatalf("write command/logout: %v", err)
			}
			if line, err := reader.ReadString('\n'); err != nil || line != "a1 NO authentication required\r\n" {
				t.Fatalf("command response = %q err = %v", line, err)
			}
			_, _ = reader.ReadString('\n')
			_, _ = reader.ReadString('\n')
			if err := <-errCh; err != nil {
				t.Fatalf("ServeConn returned error: %v", err)
			}
		})
	}
}

func TestServerRejectsMalformedSubscriptionCommand(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SUBSCRIBE\r\n")); err != nil {
		t.Fatalf("write malformed subscribe: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 BAD SUBSCRIBE requires a mailbox atom\r\n" {
		t.Fatalf("subscribe response = %q err = %v", line, err)
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
	if _, err := client.Write([]byte("a1 CREATE Projects\r\na2 LOGIN user@example.com secret\r\na3 CREATE Projects\r\na4 DELETE Projects\r\na5 RENAME Projects Archive\r\na6 CREATE INBOX\r\na7 DELETE inbox\r\na8 RENAME Inbox OldInbox\r\na9 RENAME Archive INBOX\r\n")); err != nil {
		t.Fatalf("write mailbox mutations: %v", err)
	}
	want := []string{
		"a1 NO authentication required\r\n",
		"a2 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a3 OK CREATE completed\r\n",
		"a4 OK DELETE completed\r\n",
		"a5 OK RENAME completed\r\n",
		"a6 NO CREATE cannot create INBOX\r\n",
		"a7 NO DELETE cannot delete INBOX\r\n",
		"a8 NO RENAME INBOX special semantics are not supported\r\n",
		"a9 NO RENAME cannot rename to INBOX\r\n",
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
	if _, err := client.Write([]byte("a10 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsEmptyMailboxTargets(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 CREATE \"\"\r\na3 DELETE \"\"\r\na4 RENAME \"\" Archive\r\na5 RENAME Archive \"\"\r\na6 SUBSCRIBE \"\"\r\na7 UNSUBSCRIBE \"\"\r\na8 STATUS \"\" (MESSAGES)\r\na9 SELECT \"\"\r\na10 EXAMINE \"\"\r\na11 UID COPY 7 \"\"\r\na12 UID MOVE 7 \"\"\r\na13 COPY 1 \"\"\r\na14 MOVE 1 \"\"\r\na15 LOGOUT\r\n")); err != nil {
		t.Fatalf("write empty mailbox targets: %v", err)
	}
	want := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"a2 BAD CREATE mailbox name is empty\r\n",
		"a3 BAD DELETE mailbox name is empty\r\n",
		"a4 BAD RENAME mailbox name is empty\r\n",
		"a5 BAD RENAME mailbox name is empty\r\n",
		"a6 BAD SUBSCRIBE mailbox name is empty\r\n",
		"a7 BAD UNSUBSCRIBE mailbox name is empty\r\n",
		"a8 BAD STATUS mailbox name is empty\r\n",
		"a9 BAD SELECT mailbox name is empty\r\n",
		"a10 BAD EXAMINE mailbox name is empty\r\n",
		"a11 BAD UID COPY destination mailbox name is empty\r\n",
		"a12 BAD UID MOVE destination mailbox name is empty\r\n",
		"a13 BAD COPY destination mailbox name is empty\r\n",
		"a14 BAD MOVE destination mailbox name is empty\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a15 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read empty mailbox target response: %v", err)
		}
		if line != expected {
			t.Fatalf("empty mailbox target response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsEmptyAppendMailboxTarget(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	var output strings.Builder
	writer := bufio.NewWriter(&output)
	if _, err := server.handleAppend(writer, "a1", []string{"a1", "APPEND", "", ""}, []string{""}, &imapConnState{}); err != nil {
		t.Fatalf("handleAppend returned error: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush writer: %v", err)
	}
	if got, want := output.String(), "a1 BAD APPEND mailbox name is empty\r\n"; got != want {
		t.Fatalf("append response = %q, want %q", got, want)
	}
}

func TestServerDeleteSelectedMailboxClearsSavedSearch(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	var output strings.Builder
	writer := bufio.NewWriter(&output)
	closedSubscription := false
	state := &imapConnState{
		session:               &Session{UserID: "user-1"},
		selectedMailbox:       "archive",
		selectedMessages:      3,
		selectedHighestModSeq: 77,
		selectedNoModSeq:      true,
		permanentFlags:        map[string]struct{}{FlagSeen: {}},
		readOnly:              true,
		savedSearch:           []imapSearchSavedMessage{{uid: 7, sequenceNumber: 1}},
		events:                make(chan MailboxEvent),
		cancelEvents: func() {
			closedSubscription = true
		},
	}

	done, err := server.handleDeleteMailbox(writer, "a1", []string{"a1", "DELETE", "Archive"}, state)
	if err != nil {
		t.Fatalf("handleDeleteMailbox returned error: %v", err)
	}
	if done {
		t.Fatal("handleDeleteMailbox done = true, want false")
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush delete response: %v", err)
	}
	if output.String() != "a1 OK DELETE completed\r\n" {
		t.Fatalf("delete response = %q", output.String())
	}
	if state.savedSearch != nil {
		t.Fatalf("savedSearch = %#v, want nil after selected mailbox DELETE", state.savedSearch)
	}
	if !closedSubscription || state.events != nil || state.cancelEvents != nil {
		t.Fatalf("subscription after selected mailbox DELETE = closed %t events nil %t cancel nil %t", closedSubscription, state.events == nil, state.cancelEvents == nil)
	}
	if state.selectedMailbox != "" || state.selectedMessages != 0 || state.selectedHighestModSeq != 0 || state.selectedNoModSeq || state.permanentFlags != nil || state.readOnly {
		t.Fatalf("selected state after DELETE = mailbox %q messages %d modseq %d noModSeq %t flags %#v readOnly %t", state.selectedMailbox, state.selectedMessages, state.selectedHighestModSeq, state.selectedNoModSeq, state.permanentFlags, state.readOnly)
	}
}

func TestServerRenameSelectedMailboxTracksReturnedID(t *testing.T) {
	t.Parallel()

	backendImpl := &renameSelectedBackend{renamedHighestModSeq: 22}
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: backendImpl, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	var output strings.Builder
	writer := bufio.NewWriter(&output)
	oldCanceled := false
	state := &imapConnState{
		session:               &Session{UserID: "user-1"},
		selectedMailbox:       "archive",
		selectedMessages:      3,
		selectedHighestModSeq: 10,
		permanentFlags:        map[string]struct{}{FlagSeen: {}},
		savedSearch:           []imapSearchSavedMessage{{uid: 7, sequenceNumber: 1}},
		events:                make(chan MailboxEvent),
		cancelEvents: func() {
			oldCanceled = true
		},
	}

	done, err := server.handleRenameMailbox(writer, "a1", []string{"a1", "RENAME", "Archive", "Renamed"}, state)
	if err != nil {
		t.Fatalf("handleRenameMailbox returned error: %v", err)
	}
	if done {
		t.Fatal("handleRenameMailbox done = true, want false")
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush rename response: %v", err)
	}
	if output.String() != "a1 OK RENAME completed\r\n" {
		t.Fatalf("rename response = %q", output.String())
	}
	if !oldCanceled {
		t.Fatal("old selected mailbox subscription was not canceled")
	}
	if backendImpl.renamedFrom != "archive" || backendImpl.renamedTo != "Renamed" || backendImpl.subscribed != "renamed" {
		t.Fatalf("rename backend = from %q to %q subscribed %q, want archive/Renamed/renamed", backendImpl.renamedFrom, backendImpl.renamedTo, backendImpl.subscribed)
	}
	if state.selectedMailbox != "renamed" || state.selectedHighestModSeq != 22 || state.selectedNoModSeq {
		t.Fatalf("selected state after RENAME = mailbox %q modseq %d noModSeq %t, want renamed/22/false", state.selectedMailbox, state.selectedHighestModSeq, state.selectedNoModSeq)
	}
	if state.savedSearch == nil || len(state.savedSearch) != 1 || state.savedSearch[0].uid != 7 {
		t.Fatalf("savedSearch after selected RENAME = %#v, want preserved", state.savedSearch)
	}
	if state.events == nil || state.cancelEvents == nil {
		t.Fatalf("subscription after selected RENAME = events %#v cancel nil %t", state.events, state.cancelEvents == nil)
	}
}

func TestServerRenameSelectedMailboxRefreshesNoModSeqState(t *testing.T) {
	t.Parallel()

	backendImpl := &renameSelectedBackend{renamedHighestModSeq: 0}
	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: backendImpl, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	var output strings.Builder
	writer := bufio.NewWriter(&output)
	state := &imapConnState{
		session:               &Session{UserID: "user-1"},
		selectedMailbox:       "archive",
		selectedHighestModSeq: 77,
		condstoreAware:        true,
		savedSearch:           []imapSearchSavedMessage{{uid: 9, sequenceNumber: 2}},
	}

	done, err := server.handleRenameMailbox(writer, "a1", []string{"a1", "RENAME", "Archive", "Renamed"}, state)
	if err != nil {
		t.Fatalf("handleRenameMailbox returned error: %v", err)
	}
	if done {
		t.Fatal("handleRenameMailbox done = true, want false")
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush rename response: %v", err)
	}
	if output.String() != "a1 OK RENAME completed\r\n" {
		t.Fatalf("rename response = %q", output.String())
	}
	if state.selectedMailbox != "renamed" || state.selectedHighestModSeq != 0 || !state.selectedNoModSeq {
		t.Fatalf("selected state after RENAME = mailbox %q modseq %d noModSeq %t, want renamed/0/true", state.selectedMailbox, state.selectedHighestModSeq, state.selectedNoModSeq)
	}
	if state.savedSearch == nil || len(state.savedSearch) != 1 || state.savedSearch[0].uid != 9 {
		t.Fatalf("savedSearch after selected RENAME = %#v, want preserved", state.savedSearch)
	}
	if backendImpl.subscribed != "renamed" {
		t.Fatalf("subscribed mailbox = %q, want renamed", backendImpl.subscribed)
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 STATUS inbox (MESSAGES UIDNEXT UIDVALIDITY UNSEEN SIZE)\r\n")); err != nil {
		t.Fatalf("write login/status: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* STATUS \"INBOX\" (MESSAGES 2 UIDNEXT 5 UIDVALIDITY 1 UNSEEN 1 SIZE 53)\r\n",
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

func TestServerRejectsStringStatusItemListsBeforeState(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 STATUS inbox \"(MESSAGES)\"\r\na2 STATUS inbox {10+}\r\n(MESSAGES)\r\na3 STATUS inbox (MESSAGES)\r\na4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write string status item lists: %v", err)
	}
	want := []string{
		"a1 BAD STATUS requires parenthesized item list\r\n",
		"a2 BAD STATUS requires parenthesized item list\r\n",
		"a3 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a4 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read string status item-list response: %v", err)
		}
		if line != expected {
			t.Fatalf("string status item-list response = %q, want %q", line, expected)
		}
	}
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 STATUS inbox (UIDNEXT RECENT)\r\na3 STATUS inbox (BADITEM)\r\na4 STATUS inbox MESSAGES\r\na5 STATUS inbox (UIDNEXT UIDNEXT)\r\na6 STATUS inbox ()\r\na7 STATUS inbox ( )\r\na8 STATUS inbox \"( UIDNEXT)\"\r\na9 STATUS inbox \"(UIDNEXT  RECENT)\"\r\n")); err != nil {
		t.Fatalf("write login/status: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a4 BAD STATUS requires parenthesized item list\r\n" {
		t.Fatalf("unparenthesized status line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a5 BAD STATUS item is duplicated\r\n" {
		t.Fatalf("duplicate status item line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a6 BAD STATUS requires status data items\r\n" {
		t.Fatalf("empty status item list line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a7 BAD STATUS requires status data items\r\n" {
		t.Fatalf("spaced empty status item list line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a8 BAD STATUS requires parenthesized item list\r\n" {
		t.Fatalf("padded status item list line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a9 BAD STATUS requires parenthesized item list\r\n" {
		t.Fatalf("collapsed status item list line = %q err = %v", line, err)
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

func TestServerStatusReportsHighestModSeq(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: modSeqBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 STATUS inbox (HIGHESTMODSEQ UIDNEXT)\r\n")); err != nil {
		t.Fatalf("write login/status: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "* STATUS \"INBOX\" (HIGHESTMODSEQ 9 UIDNEXT 5)\r\n" {
		t.Fatalf("status line = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 OK STATUS completed\r\n" {
		t.Fatalf("status completion = %q err = %v", line, err)
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

func TestServerStatusHighestModSeqMakesSessionCondstoreAware(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: modSeqBackend{}, AllowInsecureAuth: true})
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 STATUS inbox (HIGHESTMODSEQ)\r\na3 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/status/select: %v", err)
	}
	wantSetup := []string{
		"a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n",
		"* STATUS \"INBOX\" (HIGHESTMODSEQ 9)\r\n",
		"a2 OK STATUS completed\r\n",
		"* FLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 5] Predicted next UID\r\n",
		"* OK [HIGHESTMODSEQ 9] Highest mod-sequence\r\n",
		"* OK [PERMANENTFLAGS (\\Seen \\Flagged \\Answered \\Draft \\Deleted)] Permanent flags\r\n",
		"a3 OK [READ-WRITE] SELECT completed\r\n",
	}
	for _, expected := range wantSetup {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read setup response: %v", err)
		}
		if line != expected {
			t.Fatalf("setup response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 UID STORE 7 +FLAGS (\\Seen)\r\n")); err != nil {
		t.Fatalf("write uid store: %v", err)
	}
	wantStore := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen) MODSEQ (27))\r\n",
		"a4 OK UID STORE completed\r\n",
	}
	for _, expected := range wantStore {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read store response: %v", err)
		}
		if line != expected {
			t.Fatalf("store response = %q, want %q", line, expected)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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

func TestServerFetchFailuresUseIssuedCommandName(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fetchFailureBackend{}, AllowInsecureAuth: true})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 1 (FLAGS)\r\na4 UID FETCH 7 (FLAGS)\r\n")); err != nil {
		t.Fatalf("write failing fetch commands: %v", err)
	}
	want := []string{
		"a3 NO FETCH failed\r\n",
		"a4 NO UID FETCH failed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read failing fetch response: %v", err)
		}
		if line != expected {
			t.Fatalf("failing fetch response = %q, want %q", line, expected)
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

func TestServerRoutesMalformedUIDSubcommandsToSpecificHandlers(t *testing.T) {
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
	for i := 0; i < 8; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read login/select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID SEARCH\r\na4 UID FETCH\r\na5 UID STORE\r\na6 UID EXPUNGE\r\na7 UID COPY\r\n")); err != nil {
		t.Fatalf("write malformed uid subcommands: %v", err)
	}
	want := []string{
		"a3 BAD SEARCH requires criteria\r\n",
		"a4 BAD UID FETCH requires UID set and data items\r\n",
		"a5 BAD UID STORE requires UID, mode, and flags\r\n",
		"a6 BAD UID EXPUNGE requires UID set\r\n",
		"a7 BAD UID COPY requires UID set and destination mailbox\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read malformed uid response: %v", err)
		}
		if line != expected {
			t.Fatalf("malformed uid response = %q, want %q", line, expected)
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

func TestServerHandlesUIDFetchModSeqAfterSelect(t *testing.T) {
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
	for i := 0; i < 8; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read login/select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7 (FLAGS MODSEQ)\r\n")); err != nil {
		t.Fatalf("write uid fetch modseq: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 MODSEQ (17))\r\n",
		"a3 OK UID FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read uid fetch modseq response: %v", err)
		}
		if line != expected {
			t.Fatalf("uid fetch modseq response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read logout response: %v", err)
		}
		if line == "a4 OK LOGOUT completed\r\n" {
			break
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesUIDFetchChangedSinceAfterSelect(t *testing.T) {
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
	for i := 0; i < 8; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read login/select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7:8 (FLAGS) (CHANGEDSINCE 17)\r\na4 UID FETCH 7 (FLAGS) (CHANGEDSINCE nope)\r\na5 UID FETCH 7 (FLAGS) (CHANGEDSINCE +17)\r\na6 UID FETCH 7 (FLAGS) CHANGEDSINCE 17\r\na7 UID FETCH 7 (FLAGS) (CHANGEDSINCE 17))\r\na8 UID FETCH 7 (FLAGS) (CHANGEDSINCE 0)\r\n")); err != nil {
		t.Fatalf("write uid fetch changedsince: %v", err)
	}
	want := []string{
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41 MODSEQ (18))\r\n",
		"a3 OK UID FETCH completed\r\n",
		"a4 BAD FETCH CHANGEDSINCE modifier is invalid\r\n",
		"a5 BAD FETCH CHANGEDSINCE modifier is invalid\r\n",
		"a6 BAD FETCH CHANGEDSINCE modifier is invalid\r\n",
		"a7 BAD FETCH CHANGEDSINCE modifier is invalid\r\n",
		"a8 BAD FETCH CHANGEDSINCE modifier is invalid\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read uid fetch changedsince response: %v", err)
		}
		if line != expected {
			t.Fatalf("uid fetch changedsince response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a6 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read logout response: %v", err)
		}
		if line == "a6 OK LOGOUT completed\r\n" {
			break
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestIMAPFetchChangedSinceRejectsPaddedModSeq(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		items []string
	}{
		{name: "leading padded value", items: []string{"(FLAGS)", "(CHANGEDSINCE", " 17)"}},
		{name: "trailing padded value", items: []string{"(FLAGS)", "(CHANGEDSINCE", "17 )"}},
		{name: "quoted padded value", items: []string{"(FLAGS)", "(CHANGEDSINCE", " 17 )"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			message, ok := imapFetchDataItemsSyntaxError(tc.items)
			if !ok || message != "FETCH CHANGEDSINCE modifier is invalid" {
				t.Fatalf("syntax error = %q, %v; want CHANGEDSINCE invalid", message, ok)
			}
		})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7:8 (FLAGS RFC822.SIZE)\r\na4 UID FETCH 1:* (FLAGS RFC822.SIZE)\r\na5 UID FETCH 999:* (FLAGS RFC822.SIZE)\r\na6 UID FETCH 1:999 (FLAGS RFC822.SIZE)\r\na7 UID FETCH 1,7,999 (FLAGS RFC822.SIZE)\r\na8 UID FETCH +7 (FLAGS RFC822.SIZE)\r\n")); err != nil {
		t.Fatalf("write uid fetch set: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41)\r\n",
		"a3 OK UID FETCH completed\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41)\r\n",
		"a4 OK UID FETCH completed\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41)\r\n",
		"a5 OK UID FETCH completed\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41)\r\n",
		"a6 OK UID FETCH completed\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"a7 OK UID FETCH completed\r\n",
		"a8 BAD UID FETCH requires a positive UID set\r\n",
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
	if _, err := client.Write([]byte("a7 LOGOUT\r\n")); err != nil {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 1:* (FLAGS RFC822.SIZE)\r\na4 FETCH 999 (FLAGS)\r\na5 FETCH 1:999 (FLAGS)\r\na6 FETCH +1 (FLAGS)\r\na7 FETCH 1 ((FLAGS))\r\n")); err != nil {
		t.Fatalf("write fetch: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41)\r\n",
		"a3 OK FETCH completed\r\n",
		"a4 BAD FETCH requires a valid message sequence set\r\n",
		"a5 BAD FETCH requires a valid message sequence set\r\n",
		"a6 BAD FETCH requires a valid message sequence set\r\n",
		"a7 BAD FETCH data item list is invalid\r\n",
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
	if _, err := client.Write([]byte("a7 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsPaddedSearchCharsetsAndThreadAlgorithms(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 SEARCH CHARSET \" UTF-8 \" ALL\r\na2 SORT (DATE) \" UTF-8 \" ALL\r\na3 THREAD ORDEREDSUBJECT \" UTF-8 \" ALL\r\na4 THREAD \" ORDEREDSUBJECT \" UTF-8 ALL\r\na5 LOGOUT\r\n")); err != nil {
		t.Fatalf("write padded search controls: %v", err)
	}
	want := []string{
		"a1 BAD SEARCH criteria are unsupported\r\n",
		"a2 BAD SORT arguments are unsupported\r\n",
		"a3 BAD THREAD arguments are unsupported\r\n",
		"a4 BAD THREAD algorithm is unsupported\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a5 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read padded search control response: %v", err)
		}
		if line != expected {
			t.Fatalf("padded search control response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsMalformedSearchReturnOptionListsBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 SEARCH RETURN \" (COUNT) \" ALL\r\na2 SEARCH RETURN ( COUNT) ALL\r\na3 SEARCH RETURN (COUNT ) ALL\r\na4 SORT RETURN \" (SAVE) \" (DATE) UTF-8 ALL\r\na5 THREAD RETURN \" (SAVE) \" ORDEREDSUBJECT UTF-8 ALL\r\na6 SEARCH RETURN () ALL\r\na7 LOGOUT\r\n")); err != nil {
		t.Fatalf("write malformed search return options: %v", err)
	}
	want := []string{
		"a1 BAD SEARCH return options are unsupported\r\n",
		"a2 BAD SEARCH return options are unsupported\r\n",
		"a3 BAD SEARCH return options are unsupported\r\n",
		"a4 BAD SORT return options are unsupported\r\n",
		"a5 BAD THREAD return options are unsupported\r\n",
		"a6 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a7 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read malformed search return option response: %v", err)
		}
		if line != expected {
			t.Fatalf("malformed search return option response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsMalformedSortCriterionListsBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 SORT ( DATE) UTF-8 ALL\r\na2 SORT (DATE ) UTF-8 ALL\r\na3 SORT ((DATE)) UTF-8 ALL\r\na4 SORT (REVERSE DATE ) UTF-8 ALL\r\na5 SORT (DATE) UTF-8 ALL\r\na6 LOGOUT\r\n")); err != nil {
		t.Fatalf("write malformed sort criteria: %v", err)
	}
	want := []string{
		"a1 BAD SORT arguments are unsupported\r\n",
		"a2 BAD SORT arguments are unsupported\r\n",
		"a3 BAD SORT arguments are unsupported\r\n",
		"a4 BAD SORT arguments are unsupported\r\n",
		"a5 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a6 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read malformed sort criterion response: %v", err)
		}
		if line != expected {
			t.Fatalf("malformed sort criterion response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsPaddedFetchDataItemsBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 FETCH 1 \" (FLAGS) \"\r\na2 FETCH 1 \" FLAGS \"\r\na3 UID FETCH 7 \" (FLAGS RFC822.SIZE) \"\r\na4 FETCH 1 (FLAGS)\r\na5 LOGOUT\r\n")); err != nil {
		t.Fatalf("write padded fetch data items: %v", err)
	}
	want := []string{
		"a1 BAD FETCH data item list is invalid\r\n",
		"a2 BAD FETCH data item list is invalid\r\n",
		"a3 BAD FETCH data item list is invalid\r\n",
		"a4 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a5 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read padded fetch data item response: %v", err)
		}
		if line != expected {
			t.Fatalf("padded fetch data item response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsStringSearchReturnControlsBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 SEARCH \"RETURN\" (COUNT) ALL\r\na2 SEARCH {6+}\r\nRETURN (COUNT) ALL\r\na3 SEARCH RETURN \"(COUNT)\" ALL\r\na4 SEARCH RETURN {7+}\r\n(COUNT) ALL\r\na5 UID SEARCH \"RETURN\" (COUNT) ALL\r\na6 UID SEARCH RETURN \"(COUNT)\" ALL\r\na7 SORT \"RETURN\" (SAVE) (DATE) UTF-8 ALL\r\na8 SORT RETURN \"(SAVE)\" (DATE) UTF-8 ALL\r\na9 UID THREAD {6+}\r\nRETURN (SAVE) ORDEREDSUBJECT UTF-8 ALL\r\na10 UID THREAD RETURN {6+}\r\n(SAVE) ORDEREDSUBJECT UTF-8 ALL\r\na11 SEARCH RETURN (COUNT) ALL\r\na12 LOGOUT\r\n")); err != nil {
		t.Fatalf("write string search return controls: %v", err)
	}
	want := []string{
		"a1 BAD SEARCH requires return options atom\r\n",
		"a2 BAD SEARCH requires return options atom\r\n",
		"a3 BAD SEARCH requires parenthesized return options\r\n",
		"a4 BAD SEARCH requires parenthesized return options\r\n",
		"a5 BAD SEARCH requires return options atom\r\n",
		"a6 BAD SEARCH requires parenthesized return options\r\n",
		"a7 BAD SORT requires return options atom\r\n",
		"a8 BAD SORT requires parenthesized return options\r\n",
		"a9 BAD THREAD requires return options atom\r\n",
		"a10 BAD THREAD requires parenthesized return options\r\n",
		"a11 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a12 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read string search return control response: %v", err)
		}
		if line != expected {
			t.Fatalf("string search return control response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsStringSortCriterionListsBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 SORT \"(DATE)\" UTF-8 ALL\r\na2 SORT {6+}\r\n(DATE) UTF-8 ALL\r\na3 UID SORT \"(DATE)\" UTF-8 ALL\r\na4 UID SORT {6+}\r\n(DATE) UTF-8 ALL\r\na5 SORT RETURN (SAVE) \"(DATE)\" UTF-8 ALL\r\na6 UID SORT RETURN (SAVE) {6+}\r\n(DATE) UTF-8 ALL\r\na7 SORT (DATE) UTF-8 ALL\r\na8 LOGOUT\r\n")); err != nil {
		t.Fatalf("write string sort criterion lists: %v", err)
	}
	want := []string{
		"a1 BAD SORT requires parenthesized sort criteria\r\n",
		"a2 BAD SORT requires parenthesized sort criteria\r\n",
		"a3 BAD SORT requires parenthesized sort criteria\r\n",
		"a4 BAD SORT requires parenthesized sort criteria\r\n",
		"a5 BAD SORT requires parenthesized sort criteria\r\n",
		"a6 BAD SORT requires parenthesized sort criteria\r\n",
		"a7 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a8 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read string sort criterion-list response: %v", err)
		}
		if line != expected {
			t.Fatalf("string sort criterion-list response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerRejectsStringThreadAlgorithmBeforeAuthentication(t *testing.T) {
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
	if _, err := client.Write([]byte("a1 THREAD \"ORDEREDSUBJECT\" UTF-8 ALL\r\na2 THREAD {14+}\r\nORDEREDSUBJECT UTF-8 ALL\r\na3 UID THREAD \"ORDEREDSUBJECT\" UTF-8 ALL\r\na4 UID THREAD {14+}\r\nORDEREDSUBJECT UTF-8 ALL\r\na5 THREAD RETURN (SAVE) \"ORDEREDSUBJECT\" UTF-8 ALL\r\na6 UID THREAD RETURN (SAVE) {14+}\r\nORDEREDSUBJECT UTF-8 ALL\r\na7 THREAD ORDEREDSUBJECT UTF-8 ALL\r\na8 LOGOUT\r\n")); err != nil {
		t.Fatalf("write string thread algorithms: %v", err)
	}
	want := []string{
		"a1 BAD THREAD algorithm is unsupported\r\n",
		"a2 BAD THREAD algorithm is unsupported\r\n",
		"a3 BAD THREAD algorithm is unsupported\r\n",
		"a4 BAD THREAD algorithm is unsupported\r\n",
		"a5 BAD THREAD algorithm is unsupported\r\n",
		"a6 BAD THREAD algorithm is unsupported\r\n",
		"a7 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a8 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read string thread algorithm response: %v", err)
		}
		if line != expected {
			t.Fatalf("string thread algorithm response = %q, want %q", line, expected)
		}
	}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH ALL\r\na4 UID SEARCH ALL\r\na5 SEARCH UID 8:9\r\na6 SEARCH UNSEEN SINCE 04-May-2026 LARGER 20\r\na7 UID SEARCH ALL FROM archive SENTBEFORE 04-May-2026\r\na8 SEARCH NOT SEEN\r\na9 UID SEARCH OR FROM sender BCC hidden\r\na10 SEARCH CHARSET UTF-8 SUBJECT IMAP\r\na11 UID SEARCH CHARSET US-ASCII ALL\r\na12 SEARCH CHARSET ISO-8859-1 ALL\r\na13 SEARCH 2:*\r\na14 UID SEARCH 1:* SUBJECT Archive\r\na15 SEARCH (UNSEEN BCC hidden)\r\na16 UID SEARCH OR (SUBJECT IMAP) (BCC hidden)\r\na17 UID SEARCH MODSEQ 20\r\na18 SEARCH MODSEQ \"/flags/\\\\Seen\" all 17\r\na19 SEARCH MODSEQ \"/flags/\\\\Seen\" bogus 17\r\na20 SEARCH RETURN (MIN MAX COUNT) UNSEEN\r\na21 UID SEARCH RETURN (ALL COUNT) ALL\r\na22 SEARCH RETURN () ALL\r\na23 SEARCH RETURN (MIN) MODSEQ 20\r\na24 SEARCH RETURN (COUNT COUNT) ALL\r\na25 UID SEARCH RETURN (ALL COUNT) DELETED\r\na26 UID SEARCH UID 1:*\r\na27 UID SEARCH UID 999:*\r\na28 SEARCH (ALL)\r\na29 SEARCH ()\r\na30 SEARCH MODSEQ 20\"\r\na31 SEARCH MODSEQ \"/flags/\\\\Seen\" all\" 17\r\na32 SEARCH MODSEQ +20\r\na33 SEARCH CHARSET UTF-8\" ALL\r\na34 SEARCH +1\r\na35 UID SEARCH UID +7\r\na36 SEARCH MODSEQ 0\r\na37 SEARCH LARGER \" 20 \"\r\na38 SEARCH MODSEQ \" 20 \"\r\na39 SEARCH \" 1 \"\r\na40 UID SEARCH UID \" 7 \"\r\na41 SEARCH \"1\"\r\na42 UID SEARCH UID \"7\"\r\na43 SEARCH LARGER \"20\"\r\na44 UID SEARCH SMALLER \"20\"\r\na45 SEARCH MODSEQ \"20\"\r\na46 SEARCH MODSEQ \"/flags/\\\\Seen\" \"all\" 17\r\na47 SEARCH MODSEQ \"/flags/\\\\Seen\" all \"17\"\r\n")); err != nil {
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
		"* SEARCH 2\r\n",
		"a15 OK SEARCH completed\r\n",
		"* SEARCH 7 8\r\n",
		"a16 OK UID SEARCH completed\r\n",
		"* SEARCH 8 (MODSEQ 23)\r\n",
		"a17 OK UID SEARCH completed\r\n",
		"* SEARCH 1 2 (MODSEQ 23)\r\n",
		"a18 OK SEARCH completed\r\n",
		"a19 BAD SEARCH criteria are unsupported\r\n",
		"* ESEARCH (TAG \"a20\") MIN 2 MAX 2 COUNT 1\r\n",
		"a20 OK SEARCH completed\r\n",
		"* ESEARCH (TAG \"a21\") UID ALL 7:8 COUNT 2\r\n",
		"a21 OK UID SEARCH completed\r\n",
		"* ESEARCH (TAG \"a22\") ALL 1:2\r\n",
		"a22 OK SEARCH completed\r\n",
		"* ESEARCH (TAG \"a23\") MIN 2 MODSEQ 23\r\n",
		"a23 OK SEARCH completed\r\n",
		"a24 BAD SEARCH return options are unsupported\r\n",
		"* ESEARCH (TAG \"a25\") UID COUNT 0\r\n",
		"a25 OK UID SEARCH completed\r\n",
		"* SEARCH 7 8\r\n",
		"a26 OK UID SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a27 OK UID SEARCH completed\r\n",
		"* SEARCH 1 2\r\n",
		"a28 OK SEARCH completed\r\n",
		"a29 BAD SEARCH criteria are unsupported\r\n",
		"a30 BAD malformed command\r\n",
		"a31 BAD malformed command\r\n",
		"a32 BAD SEARCH criteria are unsupported\r\n",
		"a33 BAD malformed command\r\n",
		"a34 BAD SEARCH criteria are unsupported\r\n",
		"a35 BAD SEARCH criteria are unsupported\r\n",
		"a36 BAD SEARCH criteria are unsupported\r\n",
		"a37 BAD SEARCH criteria are unsupported\r\n",
		"a38 BAD SEARCH criteria are unsupported\r\n",
		"a39 BAD SEARCH criteria are unsupported\r\n",
		"a40 BAD SEARCH criteria are unsupported\r\n",
		"a41 BAD SEARCH criteria are unsupported\r\n",
		"a42 BAD SEARCH criteria are unsupported\r\n",
		"a43 BAD SEARCH criteria are unsupported\r\n",
		"a44 BAD SEARCH criteria are unsupported\r\n",
		"a45 BAD SEARCH criteria are unsupported\r\n",
		"a46 BAD SEARCH criteria are unsupported\r\n",
		"a47 BAD SEARCH criteria are unsupported\r\n",
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
	if _, err := client.Write([]byte("a48 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerSearchesRecentOldAndNewMessages(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: recentSearchBackend{}, AllowInsecureAuth: true})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH RECENT\r\na4 SEARCH NEW\r\na5 SEARCH OLD\r\na6 UID SEARCH RECENT\r\na7 UID SEARCH NEW\r\na8 UID SEARCH OLD\r\na9 LOGOUT\r\n")); err != nil {
		t.Fatalf("write recent search commands: %v", err)
	}
	want := []string{
		"* SEARCH 1 2\r\n",
		"a3 OK SEARCH completed\r\n",
		"* SEARCH 2\r\n",
		"a4 OK SEARCH completed\r\n",
		"* SEARCH 3\r\n",
		"a5 OK SEARCH completed\r\n",
		"* SEARCH 7 8\r\n",
		"a6 OK UID SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a7 OK UID SEARCH completed\r\n",
		"* SEARCH 9\r\n",
		"a8 OK UID SEARCH completed\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a9 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read recent search response: %v", err)
		}
		if line != expected {
			t.Fatalf("recent search response = %q, want %q", line, expected)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerHandlesCustomKeywordFlags(t *testing.T) {
	t.Parallel()

	backendImpl := &customKeywordBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	wantSelect := []string{
		"* FLAGS (\\Seen $Project)\r\n",
		"* 2 EXISTS\r\n",
		"* 0 RECENT\r\n",
		"* OK [UIDVALIDITY 1] UIDs valid\r\n",
		"* OK [UIDNEXT 9] Predicted next UID\r\n",
		"* OK [PERMANENTFLAGS (\\Seen $Project)] Permanent flags\r\n",
		"a2 OK [READ-WRITE] SELECT completed\r\n",
	}
	for _, expected := range wantSelect {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read select response: %v", err)
		}
		if line != expected {
			t.Fatalf("select response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH KEYWORD $Project\r\na4 SEARCH UNKEYWORD $Project\r\na5 UID SEARCH KEYWORD $Project\r\na6 STORE 2 +FLAGS ($Project)\r\na7 STORE 1 +FLAGS ($Other)\r\na8 LOGOUT\r\n")); err != nil {
		t.Fatalf("write keyword commands: %v", err)
	}
	want := []string{
		"* SEARCH 1\r\n",
		"a3 OK SEARCH completed\r\n",
		"* SEARCH 2\r\n",
		"a4 OK SEARCH completed\r\n",
		"* SEARCH 7\r\n",
		"a5 OK UID SEARCH completed\r\n",
		"* 2 FETCH (UID 8 FLAGS ($Project))\r\n",
		"a6 OK STORE completed\r\n",
		"a7 NO STORE flags are not permitted\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a8 OK LOGOUT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read keyword response: %v", err)
		}
		if line != expected {
			t.Fatalf("keyword response = %q, want %q", line, expected)
		}
	}
	if backendImpl.storeCalls != 1 || !reflect.DeepEqual(backendImpl.lastKeywords, []string{"$Project"}) {
		t.Fatalf("custom keyword store = calls %d keywords %#v, want one $Project", backendImpl.storeCalls, backendImpl.lastKeywords)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestParseIMAPSearchModSeqRejectsPaddedEntryTypes(t *testing.T) {
	t.Parallel()

	if threshold, consumed, ok := parseIMAPSearchModSeq([]string{"MODSEQ", "/flags/\\Seen", "ALL", "17"}); !ok || threshold != 17 || consumed != 4 {
		t.Fatalf("valid MODSEQ entry type = threshold %d consumed %d ok %v, want 17/4/true", threshold, consumed, ok)
	}
	for _, criteria := range [][]string{
		{"MODSEQ", "/flags/\\Seen", " ALL", "17"},
		{"MODSEQ", "/flags/\\Seen", "ALL ", "17"},
		{"MODSEQ", "/flags/\\Seen", " ALL ", "17"},
	} {
		if _, _, ok := parseIMAPSearchModSeq(criteria); ok {
			t.Fatalf("parseIMAPSearchModSeq(%q) accepted padded entry type", criteria)
		}
	}
}

func TestServerHandlesSortAfterSelect(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SORT (SUBJECT) UTF-8 ALL\r\na4 SORT (REVERSE DATE) UTF-8 ALL\r\na5 UID SORT (SIZE) US-ASCII ALL\r\na6 SORT (SUBJECT) UTF-8 SUBJECT Archive\r\na7 SORT (ARRIVAL) ISO-8859-1 ALL\r\na8 SORT (BOGUS) UTF-8 ALL\r\na9 SORT (SUBJECT) UTF-8\" ALL\r\na10 SORT (SUBJECT) UTF-8 +1\r\na11 SORT (reverse subject) UTF-8 ALL\r\n")); err != nil {
		t.Fatalf("write sort: %v", err)
	}
	want := []string{
		"* SORT 2 1\r\n",
		"a3 OK SORT completed\r\n",
		"* SORT 1 2\r\n",
		"a4 OK SORT completed\r\n",
		"* SORT 7 8\r\n",
		"a5 OK UID SORT completed\r\n",
		"* SORT 2\r\n",
		"a6 OK SORT completed\r\n",
		"a7 NO [BADCHARSET (US-ASCII UTF-8)] SORT charset is unsupported\r\n",
		"a8 BAD SORT arguments are unsupported\r\n",
		"a9 BAD malformed command\r\n",
		"a10 BAD SORT criteria are unsupported\r\n",
		"* SORT 1 2\r\n",
		"a11 OK SORT completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read sort response: %v", err)
		}
		if line != expected {
			t.Fatalf("sort response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a12 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestIMAPBaseSubjectFollowsRFC5256Prefixes(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"Re: [team] Fwd: Hello (fwd)":          "Hello",
		"[fwd: Re: Project Update]":            "Project Update",
		"  [list] Re:   Archive  ":             "Archive",
		"=?UTF-8?Q?Re=3A_Project_Update?=":     "Project Update",
		"=?UTF-8?B?UmU6IFByb2plY3QgVXBkYXRl?=": "Project Update",
	}
	for input, want := range tests {
		if got := imapBaseSubject(input); got != want {
			t.Fatalf("imapBaseSubject(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestServerHandlesOrderedSubjectThreadAfterSelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: threadBackend{}, AllowInsecureAuth: true})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 THREAD ORDEREDSUBJECT UTF-8 ALL\r\na4 UID THREAD ORDEREDSUBJECT US-ASCII SUBJECT Project\r\na5 THREAD ORDEREDSUBJECT ISO-8859-1 ALL\r\na6 THREAD REFERENCES UTF-8 ALL\r\na7 THREAD ORDEREDSUBJECT UTF-8\" ALL\r\na8 THREAD ORDEREDSUBJECT\" UTF-8 ALL\r\na9 THREAD ORDEREDSUBJECT UTF-8 +1\r\n")); err != nil {
		t.Fatalf("write thread commands: %v", err)
	}
	want := []string{
		"* THREAD (1)(2 (3)(4))\r\n",
		"a3 OK THREAD completed\r\n",
		"* THREAD (12 (13)(14))\r\n",
		"a4 OK UID THREAD completed\r\n",
		"a5 NO [BADCHARSET (US-ASCII UTF-8)] THREAD charset is unsupported\r\n",
		"a6 BAD THREAD algorithm is unsupported\r\n",
		"a7 BAD malformed command\r\n",
		"a8 BAD malformed command\r\n",
		"a9 BAD THREAD criteria are unsupported\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read thread response: %v", err)
		}
		if line != expected {
			t.Fatalf("thread response = %q, want %q", line, expected)
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

func TestServerSearchResDollarWorksInSortAndThreadCriteria(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: threadBackend{}, AllowInsecureAuth: true})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH RETURN (SAVE) SUBJECT Project\r\na4 SORT (DATE) UTF-8 $\r\na5 UID SORT (DATE) UTF-8 $\r\na6 THREAD ORDEREDSUBJECT UTF-8 $\r\na7 UID THREAD ORDEREDSUBJECT UTF-8 $\r\na8 SORT RETURN (SAVE) (DATE) UTF-8 SUBJECT Project\r\na9 SEARCH $\r\na10 UID THREAD RETURN (SAVE) ORDEREDSUBJECT UTF-8 SUBJECT Project\r\na11 UID SEARCH UID $\r\na12 SORT RETURN (COUNT) (DATE) UTF-8 ALL\r\na13 SEARCH $\r\n")); err != nil {
		t.Fatalf("write searchres sort/thread commands: %v", err)
	}
	want := []string{
		"a3 OK SEARCH completed\r\n",
		"* SORT 2 3 4\r\n",
		"a4 OK SORT completed\r\n",
		"* SORT 12 13 14\r\n",
		"a5 OK UID SORT completed\r\n",
		"* THREAD (2 (3)(4))\r\n",
		"a6 OK THREAD completed\r\n",
		"* THREAD (12 (13)(14))\r\n",
		"a7 OK UID THREAD completed\r\n",
		"* SORT 2 3 4\r\n",
		"a8 OK SORT completed\r\n",
		"* SEARCH 2 3 4\r\n",
		"a9 OK SEARCH completed\r\n",
		"* THREAD (12 (13)(14))\r\n",
		"a10 OK UID THREAD completed\r\n",
		"* SEARCH 12 13 14\r\n",
		"a11 OK UID SEARCH completed\r\n",
		"a12 BAD SORT return options are unsupported\r\n",
		"* SEARCH 2 3 4\r\n",
		"a13 OK SEARCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read searchres sort/thread response: %v", err)
		}
		if line != expected {
			t.Fatalf("searchres sort/thread response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a14 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerSearchResSavesAndReusesResults(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH RETURN (SAVE) UNSEEN\r\na4 SEARCH $\r\na5 UID SEARCH $ SMALLER 50\r\na6 FETCH $ (FLAGS)\r\na7 UID SEARCH UID $ SMALLER 50\r\na8 UID SEARCH RETURN (SAVE MIN) ALL\r\na9 UID FETCH $ (FLAGS)\r\na10 UID SEARCH RETURN (SAVE COUNT) DELETED\r\na11 FETCH $ (FLAGS)\r\na12 SELECT inbox\r\na13 FETCH $ (FLAGS)\r\n")); err != nil {
		t.Fatalf("write searchres commands: %v", err)
	}
	want := []string{
		"a3 OK SEARCH completed\r\n",
		"* SEARCH 2\r\n",
		"a4 OK SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a5 OK UID SEARCH completed\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41)\r\n",
		"a6 OK FETCH completed\r\n",
		"* SEARCH 8\r\n",
		"a7 OK UID SEARCH completed\r\n",
		"* ESEARCH (TAG \"a8\") UID MIN 7\r\n",
		"a8 OK UID SEARCH completed\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"a9 OK UID FETCH completed\r\n",
		"* ESEARCH (TAG \"a10\") UID COUNT 0\r\n",
		"a10 OK UID SEARCH completed\r\n",
		"a11 OK FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read searchres response: %v", err)
		}
		if line != expected {
			t.Fatalf("searchres response = %q, want %q", line, expected)
		}
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read reselect response: %v", err)
		}
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a13 OK FETCH completed\r\n" {
		t.Fatalf("fetch after select reset = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a14 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
}

func TestServerSearchResClearsSavedResultsOnSaveNo(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: searchSaveFailureBackend{}, AllowInsecureAuth: true})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH RETURN (SAVE) UNSEEN\r\na4 FETCH $ (FLAGS)\r\na5 SEARCH RETURN (SAVE) BODY missing\r\na6 FETCH $ (FLAGS)\r\n")); err != nil {
		t.Fatalf("write searchres failure commands: %v", err)
	}
	want := []string{
		"a3 OK SEARCH completed\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41)\r\n",
		"a4 OK FETCH completed\r\n",
		"a5 NO SEARCH failed\r\n",
		"a6 OK FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read searchres failure response: %v", err)
		}
		if line != expected {
			t.Fatalf("searchres failure response = %q, want %q", line, expected)
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

func TestServerHandlesFlagSearchAfterSelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: flagSearchBackend{}, AllowInsecureAuth: true})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH UNSEEN\r\na4 UID SEARCH FLAGGED\r\na5 SEARCH DRAFT\r\na6 UID SEARCH UNDRAFT\r\na7 SEARCH DELETED\r\na8 UID SEARCH UNDELETED\r\na9 SEARCH RECENT\r\na10 UID SEARCH OLD\r\na11 SEARCH NEW\r\na12 SEARCH KEYWORD custom\r\na13 UID SEARCH UNKEYWORD custom\r\na14 SEARCH KEYWORD forwarded\r\na15 UID SEARCH UNKEYWORD $Forwarded\r\na16 SEARCH KEYWORD bad%flag\r\na17 SEARCH KEYWORD custom\"\r\na18 UID SEARCH UNKEYWORD custom\"\r\na19 SEARCH KEYWORD \\Seen\r\na20 UID SEARCH UNKEYWORD bad]flag\r\na21 SEARCH KEYWORD \"custom\"\r\na22 UID SEARCH UNKEYWORD \"custom\"\r\n")); err != nil {
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
		"* SEARCH 1\r\n",
		"a14 OK SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a15 OK UID SEARCH completed\r\n",
		"a16 BAD SEARCH criteria are unsupported\r\n",
		"a17 BAD malformed command\r\n",
		"a18 BAD malformed command\r\n",
		"a19 BAD SEARCH criteria are unsupported\r\n",
		"a20 BAD SEARCH criteria are unsupported\r\n",
		"a21 BAD SEARCH criteria are unsupported\r\n",
		"a22 BAD SEARCH criteria are unsupported\r\n",
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
	if _, err := client.Write([]byte("a21 LOGOUT\r\n")); err != nil {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH SINCE 05-May-2026\r\na4 UID SEARCH BEFORE 05-May-2026\r\na5 SEARCH ON 05-May-2026\r\na6 UID SEARCH SENTON 03-May-2026\r\na7 SEARCH SENTSINCE 04-May-2026\r\na8 UID SEARCH SENTBEFORE 04-May-2026\r\na9 SEARCH SINCE 5-May-2026\r\na10 UID SEARCH SENTON 3-May-2026\r\na11 SEARCH SINCE 05-MAY-2026\r\na12 UID SEARCH SENTON 3-may-2026\r\na13 SEARCH SINCE 05-May-2026\"\r\na14 SEARCH SINCE \" 05-May-2026 \"\r\na15 SEARCH SINCE \"05-May-2026\"\r\na16 UID SEARCH SENTON \"3-May-2026\"\r\n")); err != nil {
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
		"* SEARCH 1\r\n",
		"a9 OK SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a10 OK UID SEARCH completed\r\n",
		"* SEARCH 1\r\n",
		"a11 OK SEARCH completed\r\n",
		"* SEARCH 8\r\n",
		"a12 OK UID SEARCH completed\r\n",
		"a13 BAD malformed command\r\n",
		"a14 BAD SEARCH criteria are unsupported\r\n",
		"a15 BAD SEARCH criteria are unsupported\r\n",
		"a16 BAD SEARCH criteria are unsupported\r\n",
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
	if _, err := client.Write([]byte("a17 LOGOUT\r\n")); err != nil {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH LARGER 20\r\na4 UID SEARCH SMALLER 20\r\na5 SEARCH LARGER +20\r\na6 UID SEARCH SMALLER +20\r\na7 SEARCH LARGER 020\r\na8 UID SEARCH SMALLER 020\r\na9 SEARCH LARGER 4294967296\r\na10 UID SEARCH SMALLER 4294967296\r\n")); err != nil {
		t.Fatalf("write size search: %v", err)
	}
	want := []string{
		"* SEARCH 2\r\n",
		"a3 OK SEARCH completed\r\n",
		"* SEARCH 7\r\n",
		"a4 OK UID SEARCH completed\r\n",
		"a5 BAD SEARCH criteria are unsupported\r\n",
		"a6 BAD SEARCH criteria are unsupported\r\n",
		"a7 BAD SEARCH criteria are unsupported\r\n",
		"a8 BAD SEARCH criteria are unsupported\r\n",
		"a9 BAD SEARCH criteria are unsupported\r\n",
		"a10 BAD SEARCH criteria are unsupported\r\n",
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
	if _, err := client.Write([]byte("a11 LOGOUT\r\n")); err != nil {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH SUBJECT IMAP\r\na4 UID SEARCH FROM archive\r\na5 SEARCH TO target\r\na6 UID SEARCH CC review\r\na7 SEARCH BCC hidden\r\na8 SEARCH SUBJECT \"Project \\\"Q2\\\"\"\r\na9 SEARCH SUBJECT \"\"\r\na10 UID SEARCH FROM \"\"\r\na11 SEARCH SUBJECT \"\\\"Archive\\\"\"\r\na12 SEARCH SUBJECT IMAP\"\r\n")); err != nil {
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
		"* SEARCH\r\n",
		"a8 OK SEARCH completed\r\n",
		"* SEARCH 1 2\r\n",
		"a9 OK SEARCH completed\r\n",
		"* SEARCH 7 8\r\n",
		"a10 OK UID SEARCH completed\r\n",
		"* SEARCH\r\n",
		"a11 OK SEARCH completed\r\n",
		"a12 BAD malformed command\r\n",
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
	if _, err := client.Write([]byte("a13 LOGOUT\r\n")); err != nil {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH BODY archived\r\na4 UID SEARCH TEXT Archive\r\na5 SEARCH BODY Subject\r\na6 UID SEARCH HEADER Subject Archive\r\na7 SEARCH BODY \"\"\r\na8 UID SEARCH TEXT \"\"\r\na9 SEARCH HEADER Subject \"\"\r\na10 SEARCH BODY archived\"\r\na11 UID SEARCH HEADER Subject\" Archive\r\n")); err != nil {
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
		"* SEARCH 1 2\r\n",
		"a7 OK SEARCH completed\r\n",
		"* SEARCH 7 8\r\n",
		"a8 OK UID SEARCH completed\r\n",
		"* SEARCH 2\r\n",
		"a9 OK SEARCH completed\r\n",
		"a10 BAD malformed command\r\n",
		"a11 BAD malformed command\r\n",
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
	if _, err := client.Write([]byte("a12 LOGOUT\r\n")); err != nil {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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

func TestServerFetchBodySetsSeenButPeekDoesNot(t *testing.T) {
	t.Parallel()

	backendImpl := &bodyFetchSeenBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 1 BODY.PEEK[]\r\n")); err != nil {
		t.Fatalf("write peek fetch: %v", err)
	}
	for _, expected := range []string{
		"* 1 FETCH (UID 7 FLAGS () RFC822.SIZE 11 BODY[] {11}\r\n",
		"hello world)\r\n",
		"a3 OK FETCH completed\r\n",
	} {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read peek fetch response: %v", err)
		}
		if line != expected {
			t.Fatalf("peek fetch response = %q, want %q", line, expected)
		}
	}
	if got := backendImpl.storeCalls(); got != 0 {
		t.Fatalf("store calls after BODY.PEEK = %d, want 0", got)
	}

	if _, err := client.Write([]byte("a4 FETCH 1 BODY[]\r\n")); err != nil {
		t.Fatalf("write body fetch: %v", err)
	}
	for _, expected := range []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen) RFC822.SIZE 11 BODY[] {11}\r\n",
		"hello world)\r\n",
		"a4 OK FETCH completed\r\n",
	} {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read body fetch response: %v", err)
		}
		if line != expected {
			t.Fatalf("body fetch response = %q, want %q", line, expected)
		}
	}
	if got := backendImpl.storeCalls(); got != 1 {
		t.Fatalf("store calls after BODY[] = %d, want 1", got)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 1 FAST\r\na4 FETCH 1 FULL\r\na5 FETCH 1 (FAST)\r\na6 UID FETCH 7 (FLAGS FAST)\r\n")); err != nil {
		t.Fatalf("write fetch macros: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 INTERNALDATE \"05-May-2026 12:34:56 +0900\")\r\n",
		"a3 OK FETCH completed\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 INTERNALDATE \"05-May-2026 12:34:56 +0900\" ENVELOPE (\"Tue, 05 May 2026 12:34:56 +0900\" \"Hello IMAP\" ((\"Sender\" NIL \"sender\" \"example.net\")) ((\"Sender\" NIL \"sender\" \"example.net\")) ((\"Sender\" NIL \"sender\" \"example.net\")) ((\"User\" NIL \"user\" \"example.com\")) NIL NIL NIL \"<message-7@example.net>\") BODY (\"TEXT\" \"PLAIN\" (\"CHARSET\" \"UTF-8\") NIL NIL \"7BIT\" 11 1))\r\n",
		"a4 OK FETCH completed\r\n",
		"a5 BAD FETCH macro is invalid\r\n",
		"a6 BAD FETCH macro is invalid\r\n",
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
	if _, err := client.Write([]byte("a7 LOGOUT\r\n")); err != nil {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if _, err := client.Write([]byte("a4 UID FETCH 7 BODY.PEEK[]))\r\n")); err != nil {
		t.Fatalf("write overclosed uid fetch body: %v", err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a4 BAD FETCH data item list is invalid\r\n" {
		t.Fatalf("overclosed body fetch response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a5 UID FETCH 7 RFC822\r\n")); err != nil {
		t.Fatalf("write uid fetch rfc822: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read rfc822 literal header: %v", err)
	}
	if line != "* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 RFC822 {11}\r\n" {
		t.Fatalf("rfc822 literal header = %q", line)
	}
	body = make([]byte, 11)
	if _, err := io.ReadFull(reader, body); err != nil {
		t.Fatalf("read rfc822 literal: %v", err)
	}
	if string(body) != "hello world" {
		t.Fatalf("rfc822 body = %q", body)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("rfc822 literal close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a5 OK UID FETCH completed\r\n" {
		t.Fatalf("rfc822 completion = %q err = %v", line, err)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if _, err := client.Write([]byte("a4 UID FETCH 9 BODY.PEEK[TEXT]<6.6>\r\n")); err != nil {
		t.Fatalf("write uid fetch partial text: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read partial text literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[TEXT]<6> {6}\r\n" {
		t.Fatalf("partial text literal header = %q", line)
	}
	partialText := make([]byte, 6)
	if _, err := io.ReadFull(reader, partialText); err != nil {
		t.Fatalf("read partial text literal: %v", err)
	}
	if string(partialText) != "header" {
		t.Fatalf("partial text = %q", partialText)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("partial text close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a4 OK UID FETCH completed\r\n" {
		t.Fatalf("partial completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a5 UID FETCH 9 BODY.PEEK[HEADER.FIELDS (Subject:)]\r\n")); err != nil {
		t.Fatalf("write malformed uid fetch header fields: %v", err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a5 BAD FETCH header field list is invalid\r\n" {
		t.Fatalf("malformed header fields response = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a6 UID FETCH 7 RFC822<0.5>\r\n")); err != nil {
		t.Fatalf("write uid fetch rfc822 partial body: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read rfc822 partial literal header: %v", err)
	}
	if line != "* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 RFC822<0> {5}\r\n" {
		t.Fatalf("rfc822 partial literal header = %q", line)
	}
	rfc822Partial := make([]byte, 5)
	if _, err := io.ReadFull(reader, rfc822Partial); err != nil {
		t.Fatalf("read rfc822 partial literal: %v", err)
	}
	if string(rfc822Partial) != "hello" {
		t.Fatalf("rfc822 partial body = %q", rfc822Partial)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("rfc822 partial close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a6 OK UID FETCH completed\r\n" {
		t.Fatalf("rfc822 partial completion = %q err = %v", line, err)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if _, err := client.Write([]byte("a4 UID FETCH 9 BODY.PEEK[HEADER.FIELDS.NOT (From)]<0.10>\r\n")); err != nil {
		t.Fatalf("write uid fetch partial header fields not: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read partial header fields not literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER.FIELDS.NOT (FROM)]<0> {10}\r\n" {
		t.Fatalf("partial header fields not literal header = %q", line)
	}
	partialHeader := make([]byte, 10)
	if _, err := io.ReadFull(reader, partialHeader); err != nil {
		t.Fatalf("read partial header fields not literal: %v", err)
	}
	if string(partialHeader) != "Subject: H" {
		t.Fatalf("partial header fields not = %q", partialHeader)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("partial header fields not close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a4 OK UID FETCH completed\r\n" {
		t.Fatalf("partial completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a5 UID FETCH 9 BODY.PEEK[HEADER.FIELDS ()]\r\n")); err != nil {
		t.Fatalf("write uid fetch empty header fields: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read empty header fields literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER.FIELDS ()] {2}\r\n" {
		t.Fatalf("empty header fields literal header = %q", line)
	}
	emptyHeader := make([]byte, 2)
	if _, err := io.ReadFull(reader, emptyHeader); err != nil {
		t.Fatalf("read empty header fields literal: %v", err)
	}
	if string(emptyHeader) != "\r\n" {
		t.Fatalf("empty header fields = %q", emptyHeader)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("empty header fields close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a5 OK UID FETCH completed\r\n" {
		t.Fatalf("empty header fields completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a6 UID FETCH 9 BODY.PEEK[HEADER.FIELDS ()]<0.1>\r\n")); err != nil {
		t.Fatalf("write uid fetch partial empty header fields: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read partial empty header fields literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER.FIELDS ()]<0> {1}\r\n" {
		t.Fatalf("partial empty header fields literal header = %q", line)
	}
	partialEmptyHeader := make([]byte, 1)
	if _, err := io.ReadFull(reader, partialEmptyHeader); err != nil {
		t.Fatalf("read partial empty header fields literal: %v", err)
	}
	if string(partialEmptyHeader) != "\r" {
		t.Fatalf("partial empty header fields = %q", partialEmptyHeader)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("partial empty header fields close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a6 OK UID FETCH completed\r\n" {
		t.Fatalf("partial empty header fields completion = %q err = %v", line, err)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if line != "* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41 RFC822.HEADER {20}\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER.FIELDS (SUBJECT)] {18}\r\n" {
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
	if _, err := client.Write([]byte("a4 UID FETCH 9 BODY.PEEK[HEADER.FIELDS (Subject From)]<0.14>\r\n")); err != nil {
		t.Fatalf("write uid fetch partial header fields: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read partial header fields literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER.FIELDS (SUBJECT FROM)]<0> {14}\r\n" {
		t.Fatalf("partial header fields literal header = %q", line)
	}
	partialHeader := make([]byte, 14)
	if _, err := io.ReadFull(reader, partialHeader); err != nil {
		t.Fatalf("read partial header fields literal: %v", err)
	}
	if string(partialHeader) != "Subject: Hello" {
		t.Fatalf("partial header fields = %q", partialHeader)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("partial header fields close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a4 OK UID FETCH completed\r\n" {
		t.Fatalf("partial completion = %q err = %v", line, err)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER.FIELDS.NOT (FROM)] {18}\r\n" {
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
	if _, err := client.Write([]byte("a4 UID FETCH 9 BODY.PEEK[HEADER.FIELDS.NOT ()]\r\n")); err != nil {
		t.Fatalf("write uid fetch empty header fields not: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read empty header fields not literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER.FIELDS.NOT ()] {37}\r\n" {
		t.Fatalf("empty header fields not literal header = %q", line)
	}
	fullHeader := make([]byte, 37)
	if _, err := io.ReadFull(reader, fullHeader); err != nil {
		t.Fatalf("read empty header fields not literal: %v", err)
	}
	if string(fullHeader) != "Subject: Hello\r\nFrom: sender@test\r\n\r\n" {
		t.Fatalf("empty header fields not = %q", fullHeader)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("empty header fields not close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a4 OK UID FETCH completed\r\n" {
		t.Fatalf("empty header fields not completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a5 UID FETCH 9 BODY.PEEK[HEADER.FIELDS.NOT ()]<0.10>\r\n")); err != nil {
		t.Fatalf("write uid fetch partial empty header fields not: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read partial empty header fields not literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER.FIELDS.NOT ()]<0> {10}\r\n" {
		t.Fatalf("partial empty header fields not literal header = %q", line)
	}
	partialFullHeader := make([]byte, 10)
	if _, err := io.ReadFull(reader, partialFullHeader); err != nil {
		t.Fatalf("read partial empty header fields not literal: %v", err)
	}
	if string(partialFullHeader) != "Subject: H" {
		t.Fatalf("partial empty header fields not = %q", partialFullHeader)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("partial empty header fields not close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a5 OK UID FETCH completed\r\n" {
		t.Fatalf("partial empty header fields not completion = %q err = %v", line, err)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 RFC822.TEXT {17}\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID STORE 7:8 +FLAGS (\\Seen \\Flagged \\Deleted)\r\n")); err != nil {
		t.Fatalf("write uid store: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged \\Deleted))\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged \\Deleted))\r\n",
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

func TestServerHonorsSelectedPermanentFlagsForStore(t *testing.T) {
	t.Parallel()

	backendImpl := &limitedPermanentFlagsBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID STORE 7 +FLAGS (\\Seen)\r\na4 UID STORE 7 +FLAGS (\\Deleted)\r\na5 STORE 1 +FLAGS (\\Deleted)\r\n")); err != nil {
		t.Fatalf("write uid store: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen))\r\n",
		"a3 OK UID STORE completed\r\n",
		"a4 NO UID STORE flags are not permitted\r\n",
		"a5 NO STORE flags are not permitted\r\n",
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
	if backendImpl.storeCalls != 1 {
		t.Fatalf("StoreFlags calls = %d, want 1", backendImpl.storeCalls)
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

func TestServerRejectsEmptyReplaceWhenNoPermanentFlagsAllowed(t *testing.T) {
	t.Parallel()

	backendImpl := &noPermanentFlagsBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 STORE 1 FLAGS ()\r\na4 UID STORE 7 +FLAGS ()\r\n")); err != nil {
		t.Fatalf("write empty flag-list stores: %v", err)
	}
	want := []string{
		"a3 NO STORE flags are not permitted\r\n",
		"a4 OK UID STORE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read empty permanent flag response: %v", err)
		}
		if line != expected {
			t.Fatalf("empty permanent flag response = %q, want %q", line, expected)
		}
	}
	if backendImpl.storeCalls != 0 {
		t.Fatalf("StoreFlags calls = %d, want 0", backendImpl.storeCalls)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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

func TestServerStoresForwardedKeywordFlagAfterSelect(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: forwardedPermanentFlagsBackend{}, AllowInsecureAuth: true})
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 STORE 1 +FLAGS (Forwarded)\r\n")); err != nil {
		t.Fatalf("write forwarded store: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS ($Forwarded))\r\n",
		"a3 OK STORE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read forwarded store response: %v", err)
		}
		if line != expected {
			t.Fatalf("forwarded store response = %q, want %q", line, expected)
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

func TestServerHandlesEmptyStoreFlagLists(t *testing.T) {
	t.Parallel()

	backendImpl := &emptyFlagStoreBackend{}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 STORE 1 FLAGS ()\r\na4 UID STORE 7 +FLAGS ()\r\n")); err != nil {
		t.Fatalf("write empty flag-list stores: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS ())\r\n",
		"a3 OK STORE completed\r\n",
		"a4 OK UID STORE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read empty flag-list store response: %v", err)
		}
		if line != expected {
			t.Fatalf("empty flag-list store response = %q, want %q", line, expected)
		}
	}
	if backendImpl.calls != 1 || backendImpl.lastMode != StoreFlagsReplace || !imapMessageFlagsEmpty(backendImpl.lastFlags) {
		t.Fatalf("empty flag-list backend calls=%d mode=%q flags=%#v", backendImpl.calls, backendImpl.lastMode, backendImpl.lastFlags)
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

func TestServerRejectsUnbalancedStoreFlagLists(t *testing.T) {
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
	for i := 0; i < 8; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read login/select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 STORE 1 +FLAGS \\Seen\r\na4 STORE 1 +FLAGS (\\Seen\r\na5 UID STORE 7 +FLAGS \\Seen)\r\n")); err != nil {
		t.Fatalf("write unbalanced flag-list stores: %v", err)
	}
	want := []string{
		"a3 BAD STORE flags are unsupported\r\n",
		"a4 BAD STORE flags are unsupported\r\n",
		"a5 BAD UID STORE flags are unsupported\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read unbalanced flag-list response: %v", err)
		}
		if line != expected {
			t.Fatalf("unbalanced flag-list response = %q, want %q", line, expected)
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

func TestServerStoreIncludesModSeqForCondstoreAwareSession(t *testing.T) {
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
	for i := 0; i < 8; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read login/select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7 (MODSEQ)\r\na4 UID STORE 7 +FLAGS (\\Seen)\r\n")); err != nil {
		t.Fatalf("write uid fetch/store: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 MODSEQ (17))\r\n",
		"a3 OK UID FETCH completed\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen) MODSEQ (27))\r\n",
		"a4 OK UID STORE completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read uid fetch/store response: %v", err)
		}
		if line != expected {
			t.Fatalf("uid fetch/store response = %q, want %q", line, expected)
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

func TestServerUIDStoreUnchangedSinceReturnsModified(t *testing.T) {
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
	for i := 0; i < 8; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read login/select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID STORE 7 (UNCHANGEDSINCE 27) +FLAGS (\\Seen)\r\na4 UID STORE 7:8 (UNCHANGEDSINCE 27) +FLAGS (\\Seen)\r\na5 UID STORE 7 (UNCHANGEDSINCE nope) +FLAGS (\\Seen)\r\na6 UID STORE 7 (UNCHANGEDSINCE +27) +FLAGS (\\Seen)\r\na7 UID STORE 7 UNCHANGEDSINCE 27 +FLAGS (\\Seen)\r\na8 UID STORE 7 (UNCHANGEDSINCE 27)) +FLAGS (\\Seen)\r\na9 UID STORE 7 (UNCHANGEDSINCE 0) +FLAGS (\\Seen)\r\n")); err != nil {
		t.Fatalf("write uid store unchanged since: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen) MODSEQ (27))\r\n",
		"a3 OK UID STORE completed\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen) MODSEQ (27))\r\n",
		"a4 OK [MODIFIED 8] UID STORE conditional store completed\r\n",
		"a5 BAD UID STORE UNCHANGEDSINCE modifier is invalid\r\n",
		"a6 BAD UID STORE UNCHANGEDSINCE modifier is invalid\r\n",
		"a7 BAD UID STORE UNCHANGEDSINCE modifier is invalid\r\n",
		"a8 BAD UID STORE UNCHANGEDSINCE modifier is invalid\r\n",
		"a9 OK [MODIFIED 7] UID STORE conditional store completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read uid store unchanged since response: %v", err)
		}
		if line != expected {
			t.Fatalf("uid store unchanged since response = %q, want %q", line, expected)
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

func TestServerUIDStoreModifiedFiltersStaleSummaries(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: staleStoreBackend{}, AllowInsecureAuth: true})
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
	for i := 0; i < 8; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read login/select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID STORE 7:8 (UNCHANGEDSINCE 27) +FLAGS (\\Seen)\r\n")); err != nil {
		t.Fatalf("write uid store unchanged since: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen) MODSEQ (27))\r\n",
		"a3 OK [MODIFIED 8] UID STORE conditional store completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read uid store modified response: %v", err)
		}
		if line != expected {
			t.Fatalf("uid store modified response = %q, want %q", line, expected)
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

func TestServerStoreUnchangedSinceReturnsSequenceModified(t *testing.T) {
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
	for i := 0; i < 8; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read login/select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 STORE 1 (UNCHANGEDSINCE 27) +FLAGS.SILENT (\\Seen)\r\na4 STORE 1:2 (UNCHANGEDSINCE 27) +FLAGS.SILENT (\\Seen)\r\na5 STORE 1 (UNCHANGEDSINCE +27) +FLAGS.SILENT (\\Seen)\r\na6 STORE 1 (UNCHANGEDSINCE 27)) +FLAGS.SILENT (\\Seen)\r\n")); err != nil {
		t.Fatalf("write store unchanged since: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen) MODSEQ (27))\r\n",
		"a3 OK STORE completed\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen) MODSEQ (27))\r\n",
		"a4 OK [MODIFIED 2] STORE conditional store completed\r\n",
		"a5 BAD STORE UNCHANGEDSINCE modifier is invalid\r\n",
		"a6 BAD STORE UNCHANGEDSINCE modifier is invalid\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read store unchanged since response: %v", err)
		}
		if line != expected {
			t.Fatalf("store unchanged since response = %q, want %q", line, expected)
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

func TestIMAPStoreUnchangedSinceRejectsPaddedModSeq(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		fields []string
	}{
		{name: "leading padded value", fields: []string{"(UNCHANGEDSINCE", " 27)", "+FLAGS", "(\\Seen)"}},
		{name: "trailing padded value", fields: []string{"(UNCHANGEDSINCE", "27 )", "+FLAGS", "(\\Seen)"}},
		{name: "quoted padded value", fields: []string{"(UNCHANGEDSINCE", " 27 )", "+FLAGS", "(\\Seen)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, _, ok := imapStoreUnchangedSince(tc.fields)
			if ok {
				t.Fatal("imapStoreUnchangedSince accepted padded modifier value")
			}
		})
	}
}

func TestIMAPStoreUnchangedSinceRejectsPaddedMarker(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		fields []string
	}{
		{name: "leading padded marker", fields: []string{" (UNCHANGEDSINCE", "27)", "+FLAGS", "(\\Seen)"}},
		{name: "trailing padded marker", fields: []string{"(UNCHANGEDSINCE ", "27)", "+FLAGS", "(\\Seen)"}},
		{name: "quoted padded marker", fields: []string{" (UNCHANGEDSINCE ", "27)", "+FLAGS", "(\\Seen)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if imapStoreUnchangedSincePresent(tc.fields) {
				t.Fatal("imapStoreUnchangedSincePresent accepted padded marker")
			}
			_, _, _, ok := imapStoreUnchangedSince(tc.fields)
			if ok {
				t.Fatal("imapStoreUnchangedSince accepted padded marker")
			}
		})
	}
}

func TestIMAPStoreModeRejectsPaddedModeAtoms(t *testing.T) {
	t.Parallel()

	if mode, silent, ok := imapStoreMode("+FLAGS.SILENT"); !ok || mode != StoreFlagsAdd || !silent {
		t.Fatalf("imapStoreMode(+FLAGS.SILENT) = %q %v %v, want add/silent/true", mode, silent, ok)
	}
	for _, value := range []string{" +FLAGS", "+FLAGS ", " +FLAGS ", " FLAGS.SILENT "} {
		if mode, silent, ok := imapStoreMode(value); ok {
			t.Fatalf("imapStoreMode(%q) = %q %v true, want padded mode rejection", value, mode, silent)
		}
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if _, err := parseIMAPFields("a1 LOGIN \"user\x80bad\" secret"); err == nil {
		t.Fatal("parseIMAPFields accepted non-ascii quoted string")
	}
	if _, err := parseIMAPFields(`a1 LOGIN "user"secret pass`); err == nil {
		t.Fatal("parseIMAPFields accepted adjacent quoted token")
	}
	if _, err := parseIMAPFields(`a1 LOGIN "user\nbad" secret`); err == nil {
		t.Fatal("parseIMAPFields accepted unsupported quoted escape")
	}
	if _, err := parseIMAPFields("a1 LOGIN \"user\\\rbad\" secret"); err == nil {
		t.Fatal("parseIMAPFields accepted escaped quoted control character")
	}
	if _, err := parseIMAPFields(`a1 LOGIN user"bad secret`); err == nil {
		t.Fatal("parseIMAPFields accepted quote character inside atom")
	}
	if _, err := parseIMAPFields(`a1 SEARCH SUBJECT IMAP"`); err == nil {
		t.Fatal("parseIMAPFields accepted dangling quote character at atom end")
	}
	if _, err := parseIMAPFields("a1 LOGIN user@example.com {6}"); err == nil {
		t.Fatal("parseIMAPFields accepted unsupported literal")
	}
	if _, err := parseIMAPFields("a1 LOGIN user@example.com {6+}"); err == nil {
		t.Fatal("parseIMAPFields accepted non-synchronizing literal")
	}
	if _, err := parseIMAPFieldsWithLiteral("a1 LOGIN user@example.com {6}oops", []string{"secret"}); err == nil {
		t.Fatal("parseIMAPFieldsWithLiteral accepted suffixed literal marker")
	}
	if _, err := parseIMAPFieldsWithLiteral("a1 STORE 1 (FLAGS {3})extra", []string{"foo"}); err == nil {
		t.Fatal("parseIMAPFieldsWithLiteral accepted literal marker with trailing atom data")
	}
	if _, err := parseIMAPFields("a1 STORE 1 +FLAGS (\"bad\x80flag\")"); err == nil {
		t.Fatal("parseIMAPFields accepted non-ascii parenthesized quoted string")
	}
	if _, err := parseIMAPFieldsWithLiteral("a1 NOOP", []string{"unused"}); err == nil {
		t.Fatal("parseIMAPFieldsWithLiteral accepted unused literal data")
	}
	literal := "secret value"
	fields, err := parseIMAPFieldsWithLiteral("a1 LOGIN user@example.com {12}", []string{literal})
	if err != nil {
		t.Fatalf("parseIMAPFieldsWithLiteral returned error: %v", err)
	}
	if got, want := fields, []string{"a1", "LOGIN", "user@example.com", literal}; !reflect.DeepEqual(got, want) {
		t.Fatalf("literal fields = %#v, want %#v", got, want)
	}
	fields, err = parseIMAPFieldsWithLiteral("a1 APPEND inbox {12+}", []string{literal})
	if err != nil {
		t.Fatalf("parseIMAPFieldsWithLiteral literal+ returned error: %v", err)
	}
	if got, want := fields, []string{"a1", "APPEND", "inbox", literal}; !reflect.DeepEqual(got, want) {
		t.Fatalf("literal+ fields = %#v, want %#v", got, want)
	}
	fields, err = parseIMAPFieldsWithLiteral("a1 LOGIN {16} {6}", []string{"user@example.com", "secret"})
	if err != nil {
		t.Fatalf("parseIMAPFieldsWithLiteral multiple literals returned error: %v", err)
	}
	if got, want := fields, []string{"a1", "LOGIN", "user@example.com", "secret"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("multiple literal fields = %#v, want %#v", got, want)
	}
	fields, err = parseIMAPFields(`a1 SEARCH SUBJECT "Project \"Q2\""`)
	if err != nil {
		t.Fatalf("parseIMAPFields quoted string with escaped quote returned error: %v", err)
	}
	if got, want := fields, []string{"a1", "SEARCH", "SUBJECT", `Project "Q2"`}; !reflect.DeepEqual(got, want) {
		t.Fatalf("escaped quoted fields = %#v, want %#v", got, want)
	}
	fields, err = parseIMAPFields(`a1 LIST "" ("Archive 2026" "INBOX") RETURN (STATUS (MESSAGES))`)
	if err != nil {
		t.Fatalf("parseIMAPFields spaced pattern list returned error: %v", err)
	}
	if got, want := fields, []string{"a1", "LIST", "", `("Archive 2026" "INBOX")`, "RETURN", "(STATUS", "(MESSAGES))"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("spaced pattern-list fields = %#v, want %#v", got, want)
	}
	fields, err = parseIMAPFieldsWithLiteral(`a1 LIST "" ({12} "INBOX") RETURN (STATUS (MESSAGES))`, []string{"Archive 2026"})
	if err != nil {
		t.Fatalf("parseIMAPFields literal pattern list returned error: %v", err)
	}
	if got, want := fields, []string{"a1", "LIST", "", `("Archive 2026" "INBOX")`, "RETURN", "(STATUS", "(MESSAGES))"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("literal pattern-list fields = %#v, want %#v", got, want)
	}
	if _, err := parseIMAPFieldsWithLiteral(`a1 LIST "" (Archive{12})`, []string{" 2026"}); err == nil {
		t.Fatal("parseIMAPFieldsWithLiteral accepted embedded parenthesized literal marker")
	}
	if _, err := parseIMAPFieldsWithLiteral(`a1 LIST "" ({7})`, []string{"Bad\nBox"}); err == nil {
		t.Fatal("parseIMAPFieldsWithLiteral accepted control-bearing parenthesized literal")
	}
	if _, err := parseIMAPFieldsWithLiteral(`a1 LIST "" ({7})`, []string{"Résumé"}); err == nil {
		t.Fatal("parseIMAPFieldsWithLiteral accepted non-ascii parenthesized literal")
	}
	if _, _, ok, err := imapCommandLiteralSize("a1 APPEND inbox {12}\r\n"); err != nil || !ok {
		t.Fatalf("imapCommandLiteralSize synchronizing ok = %v err = %v", ok, err)
	}
	if size, nonSync, ok, err := imapCommandLiteralSize("a1 APPEND inbox {0}\r\n"); err != nil || !ok || nonSync || size != 0 {
		t.Fatalf("imapCommandLiteralSize zero = size %d nonSync %v ok %v err %v", size, nonSync, ok, err)
	}
	if size, nonSync, ok, err := imapCommandLiteralSize("a1 APPEND inbox {12+}\r\n"); err != nil || !ok || !nonSync || size != 12 {
		t.Fatalf("imapCommandLiteralSize literal+ = size %d nonSync %v ok %v err %v", size, nonSync, ok, err)
	}
	for _, command := range []string{"a1 APPEND inbox {00}\r\n", "a1 APPEND inbox {001}\r\n", "a1 APPEND inbox {001+}\r\n", "a1 APPEND inbox {+1}\r\n", "a1 APPEND inbox {-1}\r\n", "a1 APPEND inbox {1++}\r\n"} {
		if _, _, ok, err := imapCommandLiteralSize(command); !ok || !errors.Is(err, errIMAPCommandLiteralInvalid) {
			t.Fatalf("imapCommandLiteralSize(%q) = ok %v err %v, want invalid literal size", command, ok, err)
		}
	}
}

func TestIMAPRawFieldKindTracksAfterQuotedParenthesizedList(t *testing.T) {
	t.Parallel()

	line := `a1 LIST "" ("Archive 2026" "INBOX") RETURN (STATUS (MESSAGES))`
	for _, tc := range []struct {
		index int
		want  imapRawFieldKindValue
	}{
		{index: 0, want: imapRawFieldAtom},
		{index: 1, want: imapRawFieldAtom},
		{index: 2, want: imapRawFieldQuoted},
		{index: 3, want: imapRawFieldList},
		{index: 4, want: imapRawFieldAtom},
		{index: 5, want: imapRawFieldList},
	} {
		got, ok := imapRawFieldKind(line, tc.index)
		if !ok || got != tc.want {
			t.Fatalf("imapRawFieldKind index %d = %v, %v; want %v, true", tc.index, got, ok, tc.want)
		}
	}
}

func TestIMAPQuotedStringPreservesIdentitySpacing(t *testing.T) {
	t.Parallel()

	got := imapQuotedString("Project  \"Q2\"\\Draft\t")
	want := `"Project  \"Q2\"\\Draft "`
	if got != want {
		t.Fatalf("imapQuotedString = %q, want %q", got, want)
	}
	got = imapQuotedString("Résumé \xff")
	want = `"R?sum? ?"`
	if got != want {
		t.Fatalf("imapQuotedString non-ascii = %q, want %q", got, want)
	}
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("imapQuotedString emitted line break: %q", got)
	}
}

func TestIMAPEnvelopeBoundsMetadataStrings(t *testing.T) {
	t.Parallel()

	got := imapEnvelope(MessageSummary{
		Envelope: Envelope{
			Subject: strings.Repeat("x", maxIMAPBodyMetadataTextBytes+10),
			From: []Address{{
				Name:    strings.Repeat("n", maxIMAPBodyMetadataTextBytes+10),
				Mailbox: strings.Repeat("m", maxIMAPBodyMetadataTextBytes+10),
				Host:    strings.Repeat("h", maxIMAPBodyMetadataTextBytes+10),
			}},
		},
	})
	if strings.Contains(got, strings.Repeat("x", maxIMAPBodyMetadataTextBytes+1)) {
		t.Fatalf("imapEnvelope did not bound subject: %q", got)
	}
	if strings.Contains(got, strings.Repeat("n", maxIMAPBodyMetadataTextBytes+1)) ||
		strings.Contains(got, strings.Repeat("m", maxIMAPBodyMetadataTextBytes+1)) ||
		strings.Contains(got, strings.Repeat("h", maxIMAPBodyMetadataTextBytes+1)) {
		t.Fatalf("imapEnvelope did not bound address fields: %q", got)
	}
}

func TestIMAPAddressListBoundsAddressCount(t *testing.T) {
	t.Parallel()

	addresses := make([]Address, 0, maxIMAPEnvelopeAddressCount+5)
	for i := 0; i < maxIMAPEnvelopeAddressCount+5; i++ {
		addresses = append(addresses, Address{Mailbox: fmt.Sprintf("user-%03d", i), Host: "example.net"})
	}
	got := imapAddressList(addresses)
	if strings.Contains(got, fmt.Sprintf("user-%03d", maxIMAPEnvelopeAddressCount)) {
		t.Fatalf("imapAddressList included address beyond cap: %q", got)
	}
	if !strings.Contains(got, fmt.Sprintf("user-%03d", maxIMAPEnvelopeAddressCount-1)) {
		t.Fatalf("imapAddressList omitted final capped address: %q", got)
	}
}

func TestIMAPAddressListDropsEmptyAddresses(t *testing.T) {
	t.Parallel()

	got := imapAddressList([]Address{
		{},
		{Name: "Sender", Mailbox: "sender", Host: "example.net"},
	})
	want := `(("Sender" NIL "sender" "example.net"))`
	if got != want {
		t.Fatalf("imapAddressList = %q, want empty addresses dropped", got)
	}
	if got := imapAddressList([]Address{{}}); got != "NIL" {
		t.Fatalf("imapAddressList empty-only = %q, want NIL", got)
	}
}

func TestIMAPAddressListDropsIncompleteAddresses(t *testing.T) {
	t.Parallel()

	got := imapAddressList([]Address{
		{Name: "Display Only"},
		{Name: "Mailbox Only", Mailbox: "mailbox"},
		{Name: "Host Only", Host: "example.net"},
		{Name: "Sender", Mailbox: "sender", Host: "example.net"},
	})
	want := `(("Sender" NIL "sender" "example.net"))`
	if got != want {
		t.Fatalf("imapAddressList = %q, want incomplete addresses dropped", got)
	}
}

func TestIMAPMIMEEnvelopeAddressesDropsInvalidAddrSpec(t *testing.T) {
	t.Parallel()

	got := imapMIMEEnvelopeAddresses([]messageparse.Address{
		{Name: "Display Only"},
		{Name: "Local Only", Address: "local"},
		{Name: "Sender", Address: "sender@example.net"},
	})
	want := []Address{{Name: "Sender", Mailbox: "sender", Host: "example.net"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imapMIMEEnvelopeAddresses = %#v, want invalid addr-specs dropped", got)
	}
}

func TestIMAPAddressListCapsAfterDroppingEmptyAddresses(t *testing.T) {
	t.Parallel()

	addresses := make([]Address, 0, maxIMAPEnvelopeAddressCount+2)
	addresses = append(addresses, Address{})
	for i := 0; i < maxIMAPEnvelopeAddressCount+1; i++ {
		addresses = append(addresses, Address{Mailbox: fmt.Sprintf("user-%03d", i), Host: "example.net"})
	}
	got := imapAddressList(addresses)
	if strings.Contains(got, fmt.Sprintf("user-%03d", maxIMAPEnvelopeAddressCount)) {
		t.Fatalf("imapAddressList included address beyond rendered cap: %q", got)
	}
	if !strings.Contains(got, fmt.Sprintf("user-%03d", maxIMAPEnvelopeAddressCount-1)) {
		t.Fatalf("imapAddressList capped before dropping empty placeholders: %q", got)
	}
}

func TestIMAPAppendOptionsParseFlagsAndInternalDate(t *testing.T) {
	t.Parallel()

	flags, internalDate, ok := imapAppendOptions([]string{`(\Seen`, `\Deleted)`, "05-May-2026 12:34:56 +0900"})
	if !ok {
		t.Fatal("imapAppendOptions rejected valid options")
	}
	if !flags.Read || !flags.Deleted {
		t.Fatalf("flags = %#v, want seen and deleted", flags)
	}
	wantDate := time.Date(2026, 5, 5, 12, 34, 56, 0, time.FixedZone("", 9*60*60))
	if !internalDate.Equal(wantDate) {
		t.Fatalf("internal date = %s, want %s", internalDate, wantDate)
	}
	_, paddedInternalDate, ok := imapAppendOptions([]string{" 5-May-2026 12:34:56 +0900"})
	if !ok {
		t.Fatal("imapAppendOptions rejected RFC space-padded date-day")
	}
	if !paddedInternalDate.Equal(wantDate) {
		t.Fatalf("space-padded internal date = %s, want %s", paddedInternalDate, wantDate)
	}
	_, upperMonthDate, ok := imapAppendOptions([]string{"05-MAY-2026 12:34:56 +0900"})
	if !ok {
		t.Fatal("imapAppendOptions rejected uppercase date-month")
	}
	if !upperMonthDate.Equal(wantDate) {
		t.Fatalf("uppercase month internal date = %s, want %s", upperMonthDate, wantDate)
	}
	_, lowerMonthDate, ok := imapAppendOptions([]string{" 5-may-2026 12:34:56 +0900"})
	if !ok {
		t.Fatal("imapAppendOptions rejected lowercase date-month")
	}
	if !lowerMonthDate.Equal(wantDate) {
		t.Fatalf("lowercase month internal date = %s, want %s", lowerMonthDate, wantDate)
	}
	if _, _, ok := imapAppendOptions([]string{"5-May-2026 12:34:56 +0900"}); ok {
		t.Fatal("imapAppendOptions accepted non-fixed one-digit date-day")
	}
	if _, _, ok := imapAppendOptions([]string{`(\Bad)`}); ok {
		t.Fatal("imapAppendOptions accepted unsupported flag")
	}
	if _, _, ok := imapAppendOptions([]string{"bad-date"}); ok {
		t.Fatal("imapAppendOptions accepted unsupported date")
	}
	if flags, _, ok := imapAppendOptions([]string{"()"}); !ok || !imapMessageFlagsEmpty(flags) {
		t.Fatalf("imapAppendOptions empty flag-list = %#v, %v; want empty accepted", flags, ok)
	}
	if flags, ok := imapStoreFlags("()"); !ok || !imapMessageFlagsEmpty(flags) {
		t.Fatalf("imapStoreFlags empty flag-list = %#v, %v; want empty accepted", flags, ok)
	}
	for _, value := range []string{"(\\Seen \\Seen)", "(\\Seen \\Flagged \\Seen)", "(\\Deleted \\Deleted)"} {
		if flags, ok := imapStoreFlags(value); ok {
			t.Fatalf("imapStoreFlags(%q) = %#v true, want duplicate flag rejection", value, flags)
		}
	}
	for _, value := range []string{" (\\Seen)", "(\\Seen) ", " (\\Seen) "} {
		if flags, ok := imapStoreFlags(value); ok {
			t.Fatalf("imapStoreFlags(%q) = %#v true, want padded flag-list rejection", value, flags)
		}
	}
	for _, value := range []string{"( \\Seen)", "(\\Seen )", "(\\Seen  \\Flagged)", "(\\Seen\t\\Flagged)"} {
		if flags, ok := imapStoreFlags(value); ok {
			t.Fatalf("imapStoreFlags(%q) = %#v true, want malformed inner flag-list rejection", value, flags)
		}
	}
	if got := imapAppendExistsCount(2, MessageSummary{SequenceNumber: 5}); got != 5 {
		t.Fatalf("imapAppendExistsCount with summary sequence = %d, want 5", got)
	}
	if got := imapAppendExistsCount(2, MessageSummary{}); got != 3 {
		t.Fatalf("imapAppendExistsCount fallback = %d, want 3", got)
	}
	if got := imapSummariesExistsCount(2, []MessageSummary{{SequenceNumber: 5}}); got != 5 {
		t.Fatalf("imapSummariesExistsCount with summary sequence = %d, want 5", got)
	}
	if got := imapSummariesExistsCount(2, []MessageSummary{{}, {}}); got != 4 {
		t.Fatalf("imapSummariesExistsCount fallback = %d, want 4", got)
	}
}

func TestIMAPDateMonthCanonicalizationIsASCIIOnly(t *testing.T) {
	t.Parallel()

	if day, ok := parseIMAPSearchDate("05-MAY-2026"); !ok || day.Year() != 2026 || day.Month() != time.May || day.Day() != 5 {
		t.Fatalf("parseIMAPSearchDate uppercase month = %s, %v; want canonical May", day, ok)
	}
	if _, ok := parseIMAPSearchDate("05-Máy-2026"); ok {
		t.Fatal("parseIMAPSearchDate accepted non-ASCII date-month")
	}
	if _, ok := parseIMAPAppendDate("05-Máy-2026 12:34:56 +0900"); ok {
		t.Fatal("parseIMAPAppendDate accepted non-ASCII date-month")
	}
	if _, ok := imapCanonicalMonth("máy"); ok {
		t.Fatal("imapCanonicalMonth accepted non-ASCII month token")
	}
}

func TestDecodeSASLPlainRejectsMalformedResponses(t *testing.T) {
	t.Parallel()

	valid := base64.StdEncoding.EncodeToString([]byte("\x00user@example.com\x00secret"))
	for _, value := range []string{
		"",
		"not-base64",
		" " + valid,
		valid + " ",
		"\t" + valid,
		base64.StdEncoding.EncodeToString([]byte("user@example.com\x00secret")),
		base64.StdEncoding.EncodeToString([]byte("\x00\x00secret")),
		base64.StdEncoding.EncodeToString([]byte("\x00user@example.com\x00")),
		base64.StdEncoding.EncodeToString([]byte("delegate@example.com\x00user@example.com\x00secret")),
		base64.StdEncoding.EncodeToString([]byte("\x00user@example.com\r\n\x00secret")),
		base64.StdEncoding.EncodeToString([]byte("\x00user@example.com\x00secret\r\n")),
		base64.StdEncoding.EncodeToString([]byte("\x00" + strings.Repeat("u", maxIMAPAuthIdentityBytes+1) + "\x00secret")),
		base64.StdEncoding.EncodeToString([]byte("\x00user@example.com\x00" + strings.Repeat("p", maxIMAPAuthPasswordBytes+1))),
		strings.Repeat("A", maxIMAPSASLPlainEncodedBytes),
		strings.Repeat("A", maxIMAPSASLPlainEncodedBytes+4),
	} {
		if username, password, ok := decodeSASLPlain(value); ok {
			t.Fatalf("decodeSASLPlain(%q) = %q %q true, want rejection", value, username, password)
		}
	}
}

func TestDecodeSASLPlainAcceptsMatchingAuthorizationIdentity(t *testing.T) {
	t.Parallel()

	value := base64.StdEncoding.EncodeToString([]byte("user@example.com\x00user@example.com\x00secret"))
	username, password, ok := decodeSASLPlain(value)
	if !ok {
		t.Fatal("decodeSASLPlain rejected matching authzid/authcid")
	}
	if username != "user@example.com" || password != "secret" {
		t.Fatalf("decodeSASLPlain = %q %q, want user@example.com secret", username, password)
	}
}

func TestDecodeSASLPlainPreservesCredentialSpaces(t *testing.T) {
	t.Parallel()

	value := base64.StdEncoding.EncodeToString([]byte(" user@example.com \x00 user@example.com \x00 secret "))
	username, password, ok := decodeSASLPlain(value)
	if !ok {
		t.Fatal("decodeSASLPlain rejected credentials with quoted-string-equivalent spaces")
	}
	if username != " user@example.com " || password != " secret " {
		t.Fatalf("decodeSASLPlain = %q %q, want preserved spaces", username, password)
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

	for _, value := range []string{"", "0", "7:*", "7:", "7:bad", "7: 8", "7 :8", "7, 8"} {
		if got, ok := parseIMAPUIDSet(value); ok {
			t.Fatalf("parseIMAPUIDSet(%q) = %v true, want rejection", value, got)
		}
	}
}

func TestIMAPUIDSetResponseCompactsAscendingRuns(t *testing.T) {
	t.Parallel()

	if got := imapUIDSetResponse([]UID{7, 8, 9, 11, 13, 14}); got != "7:9,11,13:14" {
		t.Fatalf("imapUIDSetResponse compacted = %q, want 7:9,11,13:14", got)
	}
	if got := imapUIDSetResponse([]UID{9, 7, 8, 12}); got != "9,7:8,12" {
		t.Fatalf("imapUIDSetResponse preserves non-contiguous order = %q, want 9,7:8,12", got)
	}
}

func TestParseIMAPUIDSetAcceptsClientScaleRanges(t *testing.T) {
	t.Parallel()

	got, ok := parseIMAPUIDSet("1:1000")
	if !ok {
		t.Fatal("parseIMAPUIDSet rejected client-scale UID range")
	}
	if len(got) != 1000 || got[0] != 1 || got[len(got)-1] != 1000 {
		t.Fatalf("UID range length/edges = %d/%d/%d, want 1000/1/1000", len(got), got[0], got[len(got)-1])
	}
	if _, ok := parseIMAPUIDSet("1:10001"); ok {
		t.Fatal("parseIMAPUIDSet accepted range above expansion cap")
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
	for _, value := range []string{"4", "1:4"} {
		if got, ok := parseIMAPSequenceSet(value, 3); ok {
			t.Fatalf("parseIMAPSequenceSet(%q, 3) = %v true, want out-of-range rejection", value, got)
		}
	}
	for _, value := range []string{"", "0", "01", "1:02", "4", "1:4", "bad", "*"} {
		if got, ok := parseIMAPSequenceSet(value, 0); ok {
			t.Fatalf("parseIMAPSequenceSet(%q, 0) = %v true, want rejection", value, got)
		}
	}
	for _, value := range []string{"01", "1:02", "1: 2", "1 :2", "1, 2"} {
		if got, ok := parseIMAPSequenceSet(value, 3); ok {
			t.Fatalf("parseIMAPSequenceSet(%q, 3) = %v true, want malformed set rejection", value, got)
		}
	}
}

func TestParseIMAPSequenceSetAcceptsClientScaleStarRanges(t *testing.T) {
	t.Parallel()

	got, ok := parseIMAPSequenceSet("1:*", 1000)
	if !ok {
		t.Fatal("parseIMAPSequenceSet rejected client-scale star range")
	}
	if len(got) != 1000 || got[0] != 1 || got[len(got)-1] != 1000 {
		t.Fatalf("sequence range length/edges = %d/%d/%d, want 1000/1/1000", len(got), got[0], got[len(got)-1])
	}
	if _, ok := parseIMAPSequenceSet("1:*", 10001); ok {
		t.Fatal("parseIMAPSequenceSet accepted range above expansion cap")
	}
}

func TestIMAPSearchResDollarRequiresExactAtom(t *testing.T) {
	t.Parallel()

	state := &imapConnState{
		savedSearch: []imapSearchSavedMessage{{uid: 7, sequenceNumber: 1}},
	}

	if got, ok := parseIMAPUIDSetForState("$", state); !ok || len(got) != 1 || got[0] != 7 {
		t.Fatalf("parseIMAPUIDSetForState($) = %v, %v; want saved UID", got, ok)
	}
	if got, ok := parseIMAPSequenceSetForState("$", 3, state); !ok || len(got) != 1 || got[0] != 1 {
		t.Fatalf("parseIMAPSequenceSetForState($) = %v, %v; want saved sequence", got, ok)
	}

	for _, value := range []string{" $", "$ ", " $ "} {
		if got, ok := parseIMAPUIDSetForState(value, state); ok {
			t.Fatalf("parseIMAPUIDSetForState(%q) = %v true, want padded $ rejection", value, got)
		}
		if got, ok := parseIMAPSequenceSetForState(value, 3, state); ok {
			t.Fatalf("parseIMAPSequenceSetForState(%q) = %v true, want padded $ rejection", value, got)
		}
		if _, _, ok := imapParseSearchPredicate([]string{"UID", value}, 3, 7, state); ok {
			t.Fatalf("imapParseSearchPredicate(UID %q) accepted padded $", value)
		}
	}
}

func TestParseIMAPSearchSize(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		value string
		want  int64
	}{
		{value: "0", want: 0},
		{value: "20", want: 20},
		{value: "4294967295", want: 4294967295},
	} {
		got, ok := parseIMAPSearchSize(tc.value)
		if !ok || got != tc.want {
			t.Fatalf("parseIMAPSearchSize(%q) = %d, %v; want %d, true", tc.value, got, ok, tc.want)
		}
	}
	for _, value := range []string{"+20", "-1", "00", "020", "20x", " 20 ", "4294967296"} {
		if got, ok := parseIMAPSearchSize(value); ok {
			t.Fatalf("parseIMAPSearchSize(%q) = %d, true; want rejection", value, got)
		}
	}
}

func TestParseIMAPModSeqValueAndValzer(t *testing.T) {
	t.Parallel()

	if got, ok := parseIMAPModSeqValue("17"); !ok || got != 17 {
		t.Fatalf("parseIMAPModSeqValue(17) = %d, %v; want 17, true", got, ok)
	}
	for _, value := range []string{"", "0", "+17", "17x", " 17 "} {
		if got, ok := parseIMAPModSeqValue(value); ok {
			t.Fatalf("parseIMAPModSeqValue(%q) = %d, true; want rejection", value, got)
		}
	}
	for _, tc := range []struct {
		value string
		want  uint64
	}{
		{value: "0", want: 0},
		{value: "17", want: 17},
	} {
		got, ok := parseIMAPModSeqValzer(tc.value)
		if !ok || got != tc.want {
			t.Fatalf("parseIMAPModSeqValzer(%q) = %d, %v; want %d, true", tc.value, got, ok, tc.want)
		}
	}
	for _, value := range []string{"", "+17", "17x", " 17 "} {
		if got, ok := parseIMAPModSeqValzer(value); ok {
			t.Fatalf("parseIMAPModSeqValzer(%q) = %d, true; want rejection", value, got)
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
	got, ok = imapFetchPartialBody([]string{"BODY.PEEK[]<4294967295.4294967295>"})
	if !ok {
		t.Fatal("imapFetchPartialBody rejected max RFC number partial")
	}
	if got.offset != 4294967295 || got.count != 4294967295 {
		t.Fatalf("max partial = %+v, want offset/count 4294967295", got)
	}
	if _, ok := imapFetchPartialBody([]string{"BODY[]"}); ok {
		t.Fatal("imapFetchPartialBody accepted full body fetch")
	}
	for _, item := range []string{"BODY.PEEK[]<+12.34>", "BODY.PEEK[]<00.34>", "BODY.PEEK[]<012.34>", "BODY.PEEK[]<12.+34>", "BODY.PEEK[]<12.0>", "BODY.PEEK[]<12.034>", "BODY.PEEK[]<12.34>BAD", "BODY.PEEK[]<4294967296.34>", "BODY.PEEK[]<12.4294967296>"} {
		if _, ok := imapFetchPartialBody([]string{item}); ok {
			t.Fatalf("imapFetchPartialBody accepted invalid partial %q", item)
		}
	}
	for _, value := range []string{"+1", "1.+2", "01", "1.02", " 1", "1 ", "1. 2"} {
		if got, ok := parseIMAPMIMEPartPath(value); ok {
			t.Fatalf("parseIMAPMIMEPartPath(%q) = %v true, want invalid path rejection", value, got)
		}
	}
	if got, ok := parseIMAPMIMEPartPath("1.4294967295"); strconv.IntSize == 64 && (!ok || !reflect.DeepEqual(got, []int{1, 4294967295})) {
		t.Fatalf("parseIMAPMIMEPartPath max RFC number = %v, %v; want valid max path", got, ok)
	}
	if got, ok := parseIMAPMIMEPartPath("1.4294967296"); ok {
		t.Fatalf("parseIMAPMIMEPartPath overflow = %v true, want rejection", got)
	}
}

func TestIMAPFetchDataItemsSyntaxRejectsUnsupportedItems(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		items []string
	}{
		{name: "unknown atom", items: []string{"BOGUS"}},
		{name: "unknown list item", items: []string{"(FLAGS", "BOGUS)"}},
		{name: "unknown section", items: []string{"BODY[BOGUS]"}},
		{name: "leading zero body part", items: []string{"BODY[01]"}},
		{name: "leading zero nested body part", items: []string{"BODY[1.02.TEXT]"}},
		{name: "leading zero partial offset", items: []string{"BODY.PEEK[]<00.34>"}},
		{name: "leading zero partial count", items: []string{"BODY.PEEK[]<12.034>"}},
		{name: "invalid partial suffix", items: []string{"BODY[HEADER.FIELDS", "(Subject)]BAD"}},
		{name: "invalid top level header field prefix", items: []string{"XBODY[HEADER.FIELDS", "(Subject)]"}},
		{name: "invalid mime part header field prefix", items: []string{"BODY.PEEK[X.HEADER.FIELDS", "(Subject)]"}},
		{name: "invalid leading zero mime part header field prefix", items: []string{"BODY.PEEK[01.HEADER.FIELDS", "(Subject)]"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			message, ok := imapFetchDataItemsSyntaxError(tc.items)
			if !ok || message != "FETCH data item is unsupported" {
				t.Fatalf("syntax error = %q, %v; want unsupported", message, ok)
			}
		})
	}
}

func TestIMAPFetchDataItemsSyntaxRejectsMalformedHeaderFieldLists(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		items []string
	}{
		{name: "space only header fields", items: []string{"BODY.PEEK[HEADER.FIELDS", "( )]"}},
		{name: "padded header fields", items: []string{"BODY.PEEK[HEADER.FIELDS", "( Subject)]"}},
		{name: "double space header fields", items: []string{"BODY.PEEK[HEADER.FIELDS", "(Subject  From)]"}},
		{name: "imap atom special header field", items: []string{"BODY.PEEK[HEADER.FIELDS", "([Subject])]"}},
		{name: "space only header fields not", items: []string{"BODY.PEEK[HEADER.FIELDS.NOT", "( )]"}},
		{name: "padded nested header fields", items: []string{"BODY.PEEK[1.HEADER.FIELDS", "( Subject)]"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			message, ok := imapFetchDataItemsSyntaxError(tc.items)
			if !ok || message != "FETCH header field list is invalid" {
				t.Fatalf("syntax error = %q, %v; want header field list invalid", message, ok)
			}
		})
	}
}

func TestIMAPHeaderFieldNameValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		want  bool
	}{
		{name: "subject", field: "Subject", want: true},
		{name: "message id", field: "Message-ID", want: true},
		{name: "underscore", field: "X_Custom", want: true},
		{name: "plus", field: "X+Custom", want: true},
		{name: "period", field: "X.Custom", want: true},
		{name: "empty", field: "", want: false},
		{name: "space", field: "Bad Field", want: false},
		{name: "colon", field: "Subject:", want: false},
		{name: "closing bracket", field: "[Subject]", want: false},
		{name: "parenthesis", field: "Subject)", want: false},
		{name: "quoted special", field: `Sub"ject`, want: false},
		{name: "backslash", field: `Sub\ject`, want: false},
		{name: "wildcard", field: "X-*", want: false},
		{name: "control", field: "Bad\tField", want: false},
		{name: "non ascii", field: "X-\x80", want: false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := imapSearchHeaderFieldNameValid(tc.field); got != tc.want {
				t.Fatalf("imapSearchHeaderFieldNameValid(%q) = %v, want %v", tc.field, got, tc.want)
			}
		})
	}
}

func TestIMAPFetchDataItemsSyntaxAcceptsSupportedItems(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		items []string
	}{
		{name: "macro", items: []string{"FULL"}},
		{name: "core list", items: []string{"(UID", "FLAGS", "RFC822.SIZE", "MODSEQ)"}},
		{name: "full body partial", items: []string{"BODY.PEEK[]<12.34>"}},
		{name: "header fields", items: []string{"BODY.PEEK[HEADER.FIELDS", "(Subject", "From)]"}},
		{name: "custom header fields", items: []string{"BODY.PEEK[HEADER.FIELDS", "(X_Custom", "X+Trace", "X.Trace)]"}},
		{name: "empty header fields", items: []string{"BODY.PEEK[HEADER.FIELDS", "()]"}},
		{name: "empty header fields not", items: []string{"BODY.PEEK[HEADER.FIELDS.NOT", "()]"}},
		{name: "header fields partial", items: []string{"BODY.PEEK[HEADER.FIELDS.NOT", "(From)]<0.10>"}},
		{name: "mime part section", items: []string{"BODY.PEEK[1.HEADER]"}},
		{name: "mime part header fields", items: []string{"BODY.PEEK[1.HEADER.FIELDS", "(Subject)]"}},
		{name: "nested mime part header fields", items: []string{"BODY.PEEK[1.2.HEADER.FIELDS.NOT", "(From)]"}},
		{name: "nested mime part partial", items: []string{"BODY.PEEK[1.2.TEXT]<0.6>"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			message, ok := imapFetchDataItemsSyntaxError(tc.items)
			if ok {
				t.Fatalf("syntax error = %q, want none", message)
			}
		})
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
	oversized := "Subject: Hi\r\n\r\n" + strings.Repeat("x", maxIMAPSearchLiteralBytes+1)
	if _, err := readIMAPSectionLiteral(strings.NewReader(oversized), false); err == nil {
		t.Fatal("readIMAPSectionLiteral accepted oversized text literal")
	}
}

func TestFilterIMAPHeaderFields(t *testing.T) {
	t.Parallel()

	got := filterIMAPHeaderFields([]byte("Subject: Hi\r\n folded\r\nFrom: sender@test\r\nTo: user@test\r\n\r\n"), []string{"subject", "to"}, false)
	want := "Subject: Hi\r\n folded\r\nTo: user@test\r\n\r\n"
	if string(got) != want {
		t.Fatalf("filtered header = %q, want %q", got, want)
	}
	got = filterIMAPHeaderFields([]byte("Subject : Bad\r\nSubject: Good\r\n\r\n"), []string{"subject"}, false)
	want = "Subject: Good\r\n\r\n"
	if string(got) != want {
		t.Fatalf("filtered malformed header name = %q, want %q", got, want)
	}
	got = filterIMAPHeaderFields([]byte("Subject: Hi\r\nFrom: sender@test\r\nTo: user@test\r\n\r\n"), []string{"from"}, true)
	want = "Subject: Hi\r\nTo: user@test\r\n\r\n"
	if string(got) != want {
		t.Fatalf("excluded header = %q, want %q", got, want)
	}
	got = filterIMAPHeaderFields([]byte("Subject: Hi\r\nFrom: sender@test\r\n\r\n"), nil, false)
	want = "\r\n"
	if string(got) != want {
		t.Fatalf("empty include header = %q, want %q", got, want)
	}
	got = filterIMAPHeaderFields([]byte("Subject: Hi\r\nFrom: sender@test\r\n\r\n"), nil, true)
	want = "Subject: Hi\r\nFrom: sender@test\r\n\r\n"
	if string(got) != want {
		t.Fatalf("empty exclude header = %q, want %q", got, want)
	}
}

func TestIMAPMailboxDisplayNameTrimsStoredRootPrefix(t *testing.T) {
	t.Parallel()

	got := imapMailboxDisplayName(Mailbox{ID: "mailbox-1", FullPath: "/Archive/2026"})
	if got != "Archive/2026" {
		t.Fatalf("display name = %q, want Archive/2026", got)
	}
}

func TestIMAPMailboxDisplayNamePreservesStoredNameSpacing(t *testing.T) {
	t.Parallel()

	got := imapMailboxDisplayName(Mailbox{ID: "mailbox-1", Name: " INBOX "})
	if got != " INBOX " {
		t.Fatalf("display name = %q, want spaced INBOX", got)
	}
}

func TestIMAPMailboxModifiedUTF7Codec(t *testing.T) {
	t.Parallel()

	name := "~peter/mail/台北/日本語"
	encoded := imapEncodeMailboxName(name)
	if encoded != "~peter/mail/&U,BTFw-/&ZeVnLIqe-" {
		t.Fatalf("encoded mailbox name = %q, want RFC 3501 example form", encoded)
	}
	decoded, ok := imapDecodeMailboxName(encoded)
	if !ok || decoded != name {
		t.Fatalf("decoded mailbox name = %q, %v, want %q true", decoded, ok, name)
	}
	ampersand, ok := imapDecodeMailboxName("Archive &- Stuff")
	if !ok || ampersand != "Archive & Stuff" {
		t.Fatalf("ampersand decode = %q, %v", ampersand, ok)
	}
	for _, bad := range []string{
		"&Jjo!",
		"&U,BTFw-&ZeVnLIqe-",
		"&AGE-",
		"Archive & Stuff",
		"日本語",
	} {
		if decoded, ok := imapDecodeMailboxName(bad); ok {
			t.Fatalf("imapDecodeMailboxName(%q) = %q true, want rejection", bad, decoded)
		}
	}
}

func TestIMAPMessageMatchesDeletedSearch(t *testing.T) {
	t.Parallel()

	deleted := MessageSummary{Flags: MessageFlags{Deleted: true}}
	active := MessageSummary{}
	if !imapMessageMatchesFlagSearch(deleted, "DELETED") {
		t.Fatal("DELETED did not match IMAP deleted flag")
	}
	if imapMessageMatchesFlagSearch(active, "DELETED") {
		t.Fatal("DELETED matched active message")
	}
	if imapMessageMatchesFlagSearch(deleted, "UNDELETED") {
		t.Fatal("UNDELETED matched IMAP deleted flag")
	}
	if !imapMessageMatchesFlagSearch(active, "UNDELETED") {
		t.Fatal("UNDELETED did not match active message")
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

func TestIMAPBodyStructureRejectsMalformedMIMETokens(t *testing.T) {
	t.Parallel()

	header := []byte("Content-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: quoted printable\r\n\r\n")
	got := imapBodyStructureFromHeader(MessageSummary{Size: 123}, header)
	want := `("TEXT" "HTML" ("CHARSET" "utf-8") NIL NIL "7BIT" 123 1 NIL NIL NIL NIL)`
	if got != want {
		t.Fatalf("bodystructure = %q, want sanitized MIME token fallback %q", got, want)
	}
}

func TestIMAPMIMETokenRejectsTspecials(t *testing.T) {
	t.Parallel()

	if got := imapMIMEToken("quoted-printable", "7BIT"); got != "QUOTED-PRINTABLE" {
		t.Fatalf("imapMIMEToken valid = %q, want QUOTED-PRINTABLE", got)
	}
	for _, value := range []string{"quoted printable", "x@y", "bad/value", "bad\x7f"} {
		if got := imapMIMEToken(value, "7BIT"); got != "7BIT" {
			t.Fatalf("imapMIMEToken(%q) = %q, want fallback", value, got)
		}
	}
}

func TestIMAPMIMETypePairFallsBackTogether(t *testing.T) {
	t.Parallel()

	mediaType, mediaSubtype := imapMIMETypePair("application", "pdf", "TEXT", "PLAIN")
	if mediaType != "APPLICATION" || mediaSubtype != "PDF" {
		t.Fatalf("imapMIMETypePair valid = %q/%q, want APPLICATION/PDF", mediaType, mediaSubtype)
	}
	mediaType, mediaSubtype = imapMIMETypePair("bad/type", "pdf", "TEXT", "PLAIN")
	if mediaType != "TEXT" || mediaSubtype != "PLAIN" {
		t.Fatalf("imapMIMETypePair invalid type = %q/%q, want TEXT/PLAIN", mediaType, mediaSubtype)
	}
	mediaType, mediaSubtype = imapMIMETypePair("application", "bad/subtype", "TEXT", "PLAIN")
	if mediaType != "TEXT" || mediaSubtype != "PLAIN" {
		t.Fatalf("imapMIMETypePair invalid subtype = %q/%q, want TEXT/PLAIN", mediaType, mediaSubtype)
	}
}

func TestIMAPMIMEBodyParameterListRejectsMalformedNames(t *testing.T) {
	t.Parallel()

	got := imapMIMEBodyParameterList(map[string]string{
		"charset": " utf-8 ",
		"bad/key": "ignored",
		"empty":   " ",
	})
	want := `("CHARSET" "utf-8")`
	if got != want {
		t.Fatalf("imapMIMEBodyParameterList = %q, want %q", got, want)
	}
	if got := imapMIMEBodyParameterList(map[string]string{"bad/key": "value"}); got != "NIL" {
		t.Fatalf("imapMIMEBodyParameterList malformed-only = %q, want NIL", got)
	}
}

func TestIMAPMIMEBodyParameterListDeduplicatesCanonicalNames(t *testing.T) {
	t.Parallel()

	got := imapMIMEBodyParameterList(map[string]string{
		"CHARSET": "utf-8",
		"charset": "iso-8859-1",
		"format":  "flowed",
	})
	want := `("CHARSET" "utf-8" "FORMAT" "flowed")`
	if got != want {
		t.Fatalf("imapMIMEBodyParameterList = %q, want canonical de-duplicated params %q", got, want)
	}
}

func TestIMAPMIMEBodyParameterListBoundsValues(t *testing.T) {
	t.Parallel()

	got := imapMIMEBodyParameterList(map[string]string{
		"filename": strings.Repeat("x", maxIMAPBodyMetadataTextBytes+10),
	})
	want := `("FILENAME" "` + strings.Repeat("x", maxIMAPBodyMetadataTextBytes) + `")`
	if got != want {
		t.Fatalf("imapMIMEBodyParameterList oversized = %q, want bounded value", got)
	}
}

func TestIMAPMIMEBodyDispositionRejectsMalformedToken(t *testing.T) {
	t.Parallel()

	if got := imapMIMEBodyDisposition(messageparse.MIMEPart{Disposition: "inline"}); got != `("INLINE" NIL)` {
		t.Fatalf("imapMIMEBodyDisposition valid = %q, want INLINE disposition", got)
	}
	got := imapMIMEBodyDisposition(messageparse.MIMEPart{
		Disposition:       "bad/value",
		DispositionParams: map[string]string{"filename": "report.pdf"},
	})
	if got != "NIL" {
		t.Fatalf("imapMIMEBodyDisposition malformed = %q, want NIL", got)
	}
}

func TestIMAPBodyMetadataNStringBoundsQuotedMetadata(t *testing.T) {
	t.Parallel()

	if got := imapBodyMetadataNString("  "); got != "NIL" {
		t.Fatalf("imapBodyMetadataNString blank = %q, want NIL", got)
	}
	got := imapBodyMetadataNString(strings.Repeat("x", maxIMAPBodyMetadataTextBytes+10))
	want := `"` + strings.Repeat("x", maxIMAPBodyMetadataTextBytes) + `"`
	if got != want {
		t.Fatalf("imapBodyMetadataNString oversized len = %d, want %d-byte quoted value", len(got), maxIMAPBodyMetadataTextBytes)
	}
}

func TestIMAPBodyMetadataTextPreservesUTF8Boundary(t *testing.T) {
	t.Parallel()

	got := imapBodyMetadataText(strings.Repeat("x", maxIMAPBodyMetadataTextBytes-1) + "\u00e9")
	if !utf8.ValidString(got) {
		t.Fatalf("imapBodyMetadataText returned invalid UTF-8")
	}
	if len(got) != maxIMAPBodyMetadataTextBytes-1 {
		t.Fatalf("imapBodyMetadataText len = %d, want truncated to UTF-8 boundary", len(got))
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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

func TestServerHandlesMessageRFC822BodyStructure(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 13 BODYSTRUCTURE\r\n")); err != nil {
		t.Fatalf("write uid fetch message bodystructure: %v", err)
	}
	bodySize := len(testMessageRFC822Body())
	messagePartSize := len("Subject: Nested\r\nFrom: nested@example.net\r\n\r\nnested body")
	nestedBodySize := len("nested body")
	want := []string{
		fmt.Sprintf("* 7 FETCH (UID 13 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODYSTRUCTURE (\"MESSAGE\" \"RFC822\" NIL NIL NIL \"7BIT\" %d (NIL \"Nested\" ((NIL NIL \"nested\" \"example.net\")) ((NIL NIL \"nested\" \"example.net\")) ((NIL NIL \"nested\" \"example.net\")) NIL NIL NIL NIL NIL) (\"TEXT\" \"PLAIN\" (\"CHARSET\" \"UTF-8\") NIL NIL \"7BIT\" %d 1 NIL NIL NIL NIL) 4 NIL NIL NIL NIL))\r\n", bodySize, messagePartSize, nestedBodySize),
		"a3 OK UID FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read message bodystructure response: %v", err)
		}
		if line != expected {
			t.Fatalf("message bodystructure response = %q, want %q", line, expected)
		}
	}
}

func TestServerHandlesMultipartMessageRFC822NestedMultipartBodyStructure(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 17 BODYSTRUCTURE\r\n")); err != nil {
		t.Fatalf("write uid fetch forwarded multipart bodystructure: %v", err)
	}
	bodySize := len(testMultipartMessageRFC822NestedMultipartBody())
	attachedMessage := testAttachedNestedMultipartMessage()
	textSize := len("see forwarded multipart")
	plainSize := len("plain attached")
	htmlSize := len("<strong>html</strong>")
	want := []string{
		fmt.Sprintf("* 11 FETCH (UID 17 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODYSTRUCTURE ((\"TEXT\" \"PLAIN\" (\"CHARSET\" \"utf-8\") NIL NIL \"7BIT\" %d 1 NIL NIL NIL NIL) (\"MESSAGE\" \"RFC822\" NIL NIL NIL \"7BIT\" %d (NIL \"Attached Multipart\" NIL NIL NIL NIL NIL NIL NIL NIL) ((\"TEXT\" \"PLAIN\" (\"CHARSET\" \"utf-8\") NIL NIL \"7BIT\" %d 1 NIL NIL NIL NIL) (\"TEXT\" \"HTML\" (\"CHARSET\" \"utf-8\") NIL NIL \"7BIT\" %d 1 NIL NIL NIL NIL) \"ALTERNATIVE\" (\"BOUNDARY\" \"attached-alt\") NIL NIL NIL) %d NIL NIL NIL NIL) \"MIXED\" (\"BOUNDARY\" \"wrap\") NIL NIL NIL))\r\n", bodySize, textSize, len(attachedMessage), plainSize, htmlSize, imapTestLineCount(attachedMessage)),
		"a3 OK UID FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read forwarded multipart bodystructure response: %v", err)
		}
		if line != expected {
			t.Fatalf("forwarded multipart bodystructure response = %q, want %q", line, expected)
		}
	}
}

func TestServerHandlesMessageRFC822SectionFetch(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 13 BODY.PEEK[1.HEADER]\r\n")); err != nil {
		t.Fatalf("write uid fetch nested message header: %v", err)
	}
	bodySize := len(testMessageRFC822Body())
	header := "Subject: Nested\r\nFrom: nested@example.net\r\n\r\n"
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read nested message header literal header: %v", err)
	}
	wantPrefix := fmt.Sprintf("* 7 FETCH (UID 13 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[1.HEADER] {%d}\r\n", bodySize, len(header))
	if line != wantPrefix {
		t.Fatalf("nested message header literal header = %q, want %q", line, wantPrefix)
	}
	literal := make([]byte, len(header))
	if _, err := io.ReadFull(reader, literal); err != nil {
		t.Fatalf("read nested message header literal: %v", err)
	}
	if string(literal) != header {
		t.Fatalf("nested message header literal = %q, want %q", literal, header)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("nested message header close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("nested message header completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 UID FETCH 13 BODY.PEEK[1.TEXT]<0.6>\r\n")); err != nil {
		t.Fatalf("write uid fetch nested message text partial: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read nested message text literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 7 FETCH (UID 13 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[1.TEXT]<0> {6}\r\n", bodySize)
	if line != wantPrefix {
		t.Fatalf("nested message text literal header = %q, want %q", line, wantPrefix)
	}
	text := make([]byte, 6)
	if _, err := io.ReadFull(reader, text); err != nil {
		t.Fatalf("read nested message text literal: %v", err)
	}
	if string(text) != "nested" {
		t.Fatalf("nested message text literal = %q", text)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("nested message text close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a4 OK UID FETCH completed\r\n" {
		t.Fatalf("nested message text completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a5 UID FETCH 13 BODY.PEEK[1.HEADER.FIELDS (SUBJECT)]\r\n")); err != nil {
		t.Fatalf("write uid fetch nested message header fields: %v", err)
	}
	headerFields := "Subject: Nested\r\n\r\n"
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read nested message header fields literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 7 FETCH (UID 13 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[1.HEADER.FIELDS (SUBJECT)] {%d}\r\n", bodySize, len(headerFields))
	if line != wantPrefix {
		t.Fatalf("nested message header fields literal header = %q, want %q", line, wantPrefix)
	}
	fieldsLiteral := make([]byte, len(headerFields))
	if _, err := io.ReadFull(reader, fieldsLiteral); err != nil {
		t.Fatalf("read nested message header fields literal: %v", err)
	}
	if string(fieldsLiteral) != headerFields {
		t.Fatalf("nested message header fields literal = %q, want %q", fieldsLiteral, headerFields)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("nested message header fields close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a5 OK UID FETCH completed\r\n" {
		t.Fatalf("nested message header fields completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a6 UID FETCH 13 BODY.PEEK[1.HEADER.FIELDS ()]\r\n")); err != nil {
		t.Fatalf("write uid fetch nested message empty header fields: %v", err)
	}
	emptyHeaderFields := "\r\n"
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read nested message empty header fields literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 7 FETCH (UID 13 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[1.HEADER.FIELDS ()] {%d}\r\n", bodySize, len(emptyHeaderFields))
	if line != wantPrefix {
		t.Fatalf("nested message empty header fields literal header = %q, want %q", line, wantPrefix)
	}
	emptyFieldsLiteral := make([]byte, len(emptyHeaderFields))
	if _, err := io.ReadFull(reader, emptyFieldsLiteral); err != nil {
		t.Fatalf("read nested message empty header fields literal: %v", err)
	}
	if string(emptyFieldsLiteral) != emptyHeaderFields {
		t.Fatalf("nested message empty header fields literal = %q, want %q", emptyFieldsLiteral, emptyHeaderFields)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("nested message empty header fields close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a6 OK UID FETCH completed\r\n" {
		t.Fatalf("nested message empty header fields completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a7 UID FETCH 13 BODY.PEEK[1.HEADER.FIELDS (SUBJECT)]<0.9>\r\n")); err != nil {
		t.Fatalf("write uid fetch nested message partial header fields: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read nested message partial header fields literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 7 FETCH (UID 13 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[1.HEADER.FIELDS (SUBJECT)]<0> {9}\r\n", bodySize)
	if line != wantPrefix {
		t.Fatalf("nested message partial header fields literal header = %q, want %q", line, wantPrefix)
	}
	partialFieldsLiteral := make([]byte, 9)
	if _, err := io.ReadFull(reader, partialFieldsLiteral); err != nil {
		t.Fatalf("read nested message partial header fields literal: %v", err)
	}
	if string(partialFieldsLiteral) != "Subject: " {
		t.Fatalf("nested message partial header fields literal = %q", partialFieldsLiteral)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("nested message partial header fields close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a7 OK UID FETCH completed\r\n" {
		t.Fatalf("nested message partial header fields completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a8 UID FETCH 13 BODY.PEEK[1.HEADER.FIELDS ()]<0.1>\r\n")); err != nil {
		t.Fatalf("write uid fetch nested message partial empty header fields: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read nested message partial empty header fields literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 7 FETCH (UID 13 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[1.HEADER.FIELDS ()]<0> {1}\r\n", bodySize)
	if line != wantPrefix {
		t.Fatalf("nested message partial empty header fields literal header = %q, want %q", line, wantPrefix)
	}
	partialEmptyFieldsLiteral := make([]byte, 1)
	if _, err := io.ReadFull(reader, partialEmptyFieldsLiteral); err != nil {
		t.Fatalf("read nested message partial empty header fields literal: %v", err)
	}
	if string(partialEmptyFieldsLiteral) != "\r" {
		t.Fatalf("nested message partial empty header fields literal = %q", partialEmptyFieldsLiteral)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("nested message partial empty header fields close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a8 OK UID FETCH completed\r\n" {
		t.Fatalf("nested message partial empty header fields completion = %q err = %v", line, err)
	}
}

func TestServerHandlesMessageRFC822NestedMultipartPartFetch(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 15 BODY.PEEK[1.2]\r\n")); err != nil {
		t.Fatalf("write uid fetch nested multipart message part: %v", err)
	}
	bodySize := len(testMessageRFC822NestedMultipartBody())
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read nested multipart message part literal header: %v", err)
	}
	wantPrefix := fmt.Sprintf("* 9 FETCH (UID 15 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[1.2] {11}\r\n", bodySize)
	if line != wantPrefix {
		t.Fatalf("nested multipart message part literal header = %q, want %q", line, wantPrefix)
	}
	literal := make([]byte, 11)
	if _, err := io.ReadFull(reader, literal); err != nil {
		t.Fatalf("read nested multipart message part literal: %v", err)
	}
	if string(literal) != "<b>html</b>" {
		t.Fatalf("nested multipart message part literal = %q", literal)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("nested multipart message part close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("nested multipart message part completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 UID FETCH 15 BODY.PEEK[1.2.MIME]\r\n")); err != nil {
		t.Fatalf("write uid fetch nested multipart message part mime: %v", err)
	}
	header := "Content-Type: text/html; charset=utf-8\r\n\r\n"
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read nested multipart message part mime literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 9 FETCH (UID 15 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[1.2.MIME] {%d}\r\n", bodySize, len(header))
	if line != wantPrefix {
		t.Fatalf("nested multipart message part mime literal header = %q, want %q", line, wantPrefix)
	}
	mimeLiteral := make([]byte, len(header))
	if _, err := io.ReadFull(reader, mimeLiteral); err != nil {
		t.Fatalf("read nested multipart message part mime literal: %v", err)
	}
	if string(mimeLiteral) != header {
		t.Fatalf("nested multipart message part mime literal = %q, want %q", mimeLiteral, header)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("nested multipart message part mime close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a4 OK UID FETCH completed\r\n" {
		t.Fatalf("nested multipart message part mime completion = %q err = %v", line, err)
	}
}

func TestServerHandlesMalformedMessageRFC822SectionFetch(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 16 BODY.PEEK[1.HEADER]\r\n")); err != nil {
		t.Fatalf("write uid fetch malformed message header: %v", err)
	}
	bodySize := len(testMalformedMessageRFC822Body())
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read malformed message header literal header: %v", err)
	}
	wantPrefix := fmt.Sprintf("* 10 FETCH (UID 16 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[1.HEADER] {2}\r\n", bodySize)
	if line != wantPrefix {
		t.Fatalf("malformed message header literal header = %q, want %q", line, wantPrefix)
	}
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		t.Fatalf("read malformed message header literal: %v", err)
	}
	if string(header) != "\r\n" {
		t.Fatalf("malformed message header literal = %q", header)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("malformed message header close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("malformed message header completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 UID FETCH 16 BODY.PEEK[1.TEXT]\r\n")); err != nil {
		t.Fatalf("write uid fetch malformed message text: %v", err)
	}
	textLiteral := "not a header line\r\nstill raw"
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read malformed message text literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 10 FETCH (UID 16 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[1.TEXT] {%d}\r\n", bodySize, len(textLiteral))
	if line != wantPrefix {
		t.Fatalf("malformed message text literal header = %q, want %q", line, wantPrefix)
	}
	text := make([]byte, len(textLiteral))
	if _, err := io.ReadFull(reader, text); err != nil {
		t.Fatalf("read malformed message text literal: %v", err)
	}
	if string(text) != textLiteral {
		t.Fatalf("malformed message text literal = %q, want %q", text, textLiteral)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("malformed message text close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a4 OK UID FETCH completed\r\n" {
		t.Fatalf("malformed message text completion = %q err = %v", line, err)
	}
}

func TestServerHandlesMultipartMessageRFC822SectionFetch(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 14 BODY.PEEK[2.HEADER]\r\n")); err != nil {
		t.Fatalf("write uid fetch attached message header: %v", err)
	}
	bodySize := len(testMultipartMessageRFC822Body())
	header := "Subject: Attached\r\nFrom: attached@example.net\r\n\r\n"
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read attached message header literal header: %v", err)
	}
	wantPrefix := fmt.Sprintf("* 8 FETCH (UID 14 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[2.HEADER] {%d}\r\n", bodySize, len(header))
	if line != wantPrefix {
		t.Fatalf("attached message header literal header = %q, want %q", line, wantPrefix)
	}
	literal := make([]byte, len(header))
	if _, err := io.ReadFull(reader, literal); err != nil {
		t.Fatalf("read attached message header literal: %v", err)
	}
	if string(literal) != header {
		t.Fatalf("attached message header literal = %q, want %q", literal, header)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("attached message header close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("attached message header completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 UID FETCH 14 BODY.PEEK[2.HEADER.FIELDS.NOT (FROM)]\r\n")); err != nil {
		t.Fatalf("write uid fetch attached message excluded header fields: %v", err)
	}
	headerFieldsNot := "Subject: Attached\r\n\r\n"
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read attached message excluded header fields literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 8 FETCH (UID 14 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[2.HEADER.FIELDS.NOT (FROM)] {%d}\r\n", bodySize, len(headerFieldsNot))
	if line != wantPrefix {
		t.Fatalf("attached message excluded header fields literal header = %q, want %q", line, wantPrefix)
	}
	fieldsLiteral := make([]byte, len(headerFieldsNot))
	if _, err := io.ReadFull(reader, fieldsLiteral); err != nil {
		t.Fatalf("read attached message excluded header fields literal: %v", err)
	}
	if string(fieldsLiteral) != headerFieldsNot {
		t.Fatalf("attached message excluded header fields literal = %q, want %q", fieldsLiteral, headerFieldsNot)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("attached message excluded header fields close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a4 OK UID FETCH completed\r\n" {
		t.Fatalf("attached message excluded header fields completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a5 UID FETCH 14 BODY.PEEK[2.HEADER.FIELDS.NOT ()]\r\n")); err != nil {
		t.Fatalf("write uid fetch attached message empty excluded header fields: %v", err)
	}
	fullHeader := "Subject: Attached\r\nFrom: attached@example.net\r\n\r\n"
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read attached message empty excluded header fields literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 8 FETCH (UID 14 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[2.HEADER.FIELDS.NOT ()] {%d}\r\n", bodySize, len(fullHeader))
	if line != wantPrefix {
		t.Fatalf("attached message empty excluded header fields literal header = %q, want %q", line, wantPrefix)
	}
	fullHeaderLiteral := make([]byte, len(fullHeader))
	if _, err := io.ReadFull(reader, fullHeaderLiteral); err != nil {
		t.Fatalf("read attached message empty excluded header fields literal: %v", err)
	}
	if string(fullHeaderLiteral) != fullHeader {
		t.Fatalf("attached message empty excluded header fields literal = %q, want %q", fullHeaderLiteral, fullHeader)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("attached message empty excluded header fields close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a5 OK UID FETCH completed\r\n" {
		t.Fatalf("attached message empty excluded header fields completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a6 UID FETCH 14 BODY.PEEK[2.HEADER.FIELDS.NOT ()]<0.10>\r\n")); err != nil {
		t.Fatalf("write uid fetch attached message partial empty excluded header fields: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read attached message partial empty excluded header fields literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 8 FETCH (UID 14 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[2.HEADER.FIELDS.NOT ()]<0> {10}\r\n", bodySize)
	if line != wantPrefix {
		t.Fatalf("attached message partial empty excluded header fields literal header = %q, want %q", line, wantPrefix)
	}
	partialFullHeaderLiteral := make([]byte, 10)
	if _, err := io.ReadFull(reader, partialFullHeaderLiteral); err != nil {
		t.Fatalf("read attached message partial empty excluded header fields literal: %v", err)
	}
	if string(partialFullHeaderLiteral) != "Subject: A" {
		t.Fatalf("attached message partial empty excluded header fields literal = %q", partialFullHeaderLiteral)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("attached message partial empty excluded header fields close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a6 OK UID FETCH completed\r\n" {
		t.Fatalf("attached message partial empty excluded header fields completion = %q err = %v", line, err)
	}
}

func TestServerHandlesMultipartMessageRFC822NestedMultipartPartFetch(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 17 BODY.PEEK[2.2]\r\n")); err != nil {
		t.Fatalf("write uid fetch multipart attached message nested part: %v", err)
	}
	bodySize := len(testMultipartMessageRFC822NestedMultipartBody())
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read multipart attached message nested part literal header: %v", err)
	}
	htmlLiteral := "<strong>html</strong>"
	wantPrefix := fmt.Sprintf("* 11 FETCH (UID 17 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[2.2] {%d}\r\n", bodySize, len(htmlLiteral))
	if line != wantPrefix {
		t.Fatalf("multipart attached message nested part literal header = %q, want %q", line, wantPrefix)
	}
	literal := make([]byte, len(htmlLiteral))
	if _, err := io.ReadFull(reader, literal); err != nil {
		t.Fatalf("read multipart attached message nested part literal: %v", err)
	}
	if string(literal) != htmlLiteral {
		t.Fatalf("multipart attached message nested part literal = %q", literal)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("multipart attached message nested part close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("multipart attached message nested part completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 UID FETCH 17 BODY.PEEK[2.2.MIME]\r\n")); err != nil {
		t.Fatalf("write uid fetch multipart attached message nested part mime: %v", err)
	}
	header := "Content-Type: text/html; charset=utf-8\r\n\r\n"
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read multipart attached message nested part mime literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 11 FETCH (UID 17 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[2.2.MIME] {%d}\r\n", bodySize, len(header))
	if line != wantPrefix {
		t.Fatalf("multipart attached message nested part mime literal header = %q, want %q", line, wantPrefix)
	}
	mimeLiteral := make([]byte, len(header))
	if _, err := io.ReadFull(reader, mimeLiteral); err != nil {
		t.Fatalf("read multipart attached message nested part mime literal: %v", err)
	}
	if string(mimeLiteral) != header {
		t.Fatalf("multipart attached message nested part mime literal = %q, want %q", mimeLiteral, header)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("multipart attached message nested part mime close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a4 OK UID FETCH completed\r\n" {
		t.Fatalf("multipart attached message nested part mime completion = %q err = %v", line, err)
	}
}

func TestServerHandlesMultipartPartFetch(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 11 BODY[1]\r\n")); err != nil {
		t.Fatalf("write uid fetch first part: %v", err)
	}
	bodySize := len(testMultipartBody())
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read first part literal header: %v", err)
	}
	wantPrefix := fmt.Sprintf("* 5 FETCH (UID 11 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[1] {5}\r\n", bodySize)
	if line != wantPrefix {
		t.Fatalf("first part literal header = %q, want %q", line, wantPrefix)
	}
	firstPart := make([]byte, 5)
	if _, err := io.ReadFull(reader, firstPart); err != nil {
		t.Fatalf("read first part literal: %v", err)
	}
	if string(firstPart) != "hello" {
		t.Fatalf("first part literal = %q", firstPart)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("first part close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("first part completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a4 UID FETCH 11 BODY[2]\r\n")); err != nil {
		t.Fatalf("write uid fetch part: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read part literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 5 FETCH (UID 11 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[2] {12}\r\n", bodySize)
	if line != wantPrefix {
		t.Fatalf("part literal header = %q, want %q", line, wantPrefix)
	}
	literal := make([]byte, 12)
	if _, err := io.ReadFull(reader, literal); err != nil {
		t.Fatalf("read part literal: %v", err)
	}
	if string(literal) != "UEZGREFUQQ==" {
		t.Fatalf("part literal = %q", literal)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("part close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a4 OK UID FETCH completed\r\n" {
		t.Fatalf("completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a5 UID FETCH 11 BODY.PEEK[2]<4.4>\r\n")); err != nil {
		t.Fatalf("write uid fetch partial part: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read partial part literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 5 FETCH (UID 11 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[2]<4> {4}\r\n", bodySize)
	if line != wantPrefix {
		t.Fatalf("partial part literal header = %q, want %q", line, wantPrefix)
	}
	partialPart := make([]byte, 4)
	if _, err := io.ReadFull(reader, partialPart); err != nil {
		t.Fatalf("read partial part literal: %v", err)
	}
	if string(partialPart) != "REFU" {
		t.Fatalf("partial part literal = %q", partialPart)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("partial part close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a5 OK UID FETCH completed\r\n" {
		t.Fatalf("partial part completion = %q err = %v", line, err)
	}
	if _, err := client.Write([]byte("a6 UID FETCH 11 BODY.PEEK[1.MIME]\r\n")); err != nil {
		t.Fatalf("write uid fetch part mime: %v", err)
	}
	header := "Content-Transfer-Encoding: 7bit\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n"
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read part mime literal header: %v", err)
	}
	wantPrefix = fmt.Sprintf("* 5 FETCH (UID 11 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[1.MIME] {%d}\r\n", bodySize, len(header))
	if line != wantPrefix {
		t.Fatalf("part mime literal header = %q, want %q", line, wantPrefix)
	}
	mimeLiteral := make([]byte, len(header))
	if _, err := io.ReadFull(reader, mimeLiteral); err != nil {
		t.Fatalf("read part mime literal: %v", err)
	}
	if string(mimeLiteral) != header {
		t.Fatalf("part mime literal = %q", mimeLiteral)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("part mime close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a6 OK UID FETCH completed\r\n" {
		t.Fatalf("part mime completion = %q err = %v", line, err)
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

func TestServerHandlesNestedMultipartPartFetch(t *testing.T) {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 12 BODY[1.2]\r\n")); err != nil {
		t.Fatalf("write uid fetch nested part: %v", err)
	}
	bodySize := len(testNestedMultipartBody())
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read nested part literal header: %v", err)
	}
	wantPrefix := fmt.Sprintf("* 6 FETCH (UID 12 FLAGS (\\Seen \\Flagged) RFC822.SIZE %d BODY[1.2] {11}\r\n", bodySize)
	if line != wantPrefix {
		t.Fatalf("nested part literal header = %q, want %q", line, wantPrefix)
	}
	literal := make([]byte, 11)
	if _, err := io.ReadFull(reader, literal); err != nil {
		t.Fatalf("read nested part literal: %v", err)
	}
	if string(literal) != "<b>html</b>" {
		t.Fatalf("nested part literal = %q", literal)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("nested part close = %q err = %v", line, err)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-EXTENDED LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT] LOGIN completed\r\n" {
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

type authFailureBackend struct {
	fakeBackend
}

func (authFailureBackend) Authenticate(context.Context, string, string) (Session, error) {
	return Session{}, fmt.Errorf("invalid credentials")
}

type specialUseBackend struct {
	fakeBackend
}

type childMailboxBackend struct {
	fakeBackend
}

type nestedMailboxBackend struct {
	fakeBackend
}

type subscriptionBackend struct {
	fakeBackend
}

type listStatusBackend struct {
	fakeBackend
}

type spacedListPatternBackend struct {
	fakeBackend
}

type unicodeMailboxBackend struct {
	fakeBackend
}

func (unicodeMailboxBackend) ListMailboxes(context.Context, ListMailboxesRequest) ([]Mailbox, error) {
	return []Mailbox{
		{ID: "unicode", Name: "~peter/mail/台北/日本語", UIDValidity: 1, UIDNext: 1},
	}, nil
}

type spacedMailboxBackend struct {
	fakeBackend
}

func (spacedMailboxBackend) ListMailboxes(context.Context, ListMailboxesRequest) ([]Mailbox, error) {
	return []Mailbox{
		{ID: "project-q2", Name: "Project  Q2", UIDValidity: 1, UIDNext: 1},
		{ID: "archive", Name: "Archive\r\n2026", UIDValidity: 2, UIDNext: 1},
	}, nil
}

func (subscriptionBackend) ListSubscribedMailboxes(context.Context, ListMailboxesRequest) ([]MailboxSubscription, error) {
	return []MailboxSubscription{
		{Name: "INBOX", Mailbox: Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2}, Exists: true},
		{Name: "Retired"},
	}, nil
}

type hierarchySubscriptionBackend struct {
	fakeBackend
}

func (hierarchySubscriptionBackend) ListSubscribedMailboxes(context.Context, ListMailboxesRequest) ([]MailboxSubscription, error) {
	return []MailboxSubscription{
		{Name: "Projects/2026"},
	}, nil
}

type subscriptionCommandBackend struct {
	fakeBackend
	subscribed   MailboxID
	unsubscribed MailboxID
}

func (b *subscriptionCommandBackend) SubscribeMailbox(_ context.Context, _ UserID, mailboxID MailboxID) (MailboxSubscription, error) {
	b.subscribed = mailboxID
	return MailboxSubscription{Name: string(mailboxID), Mailbox: Mailbox{ID: mailboxID, Name: string(mailboxID)}, Exists: true}, nil
}

func (b *subscriptionCommandBackend) UnsubscribeMailbox(_ context.Context, _ UserID, mailboxID MailboxID) error {
	b.unsubscribed = mailboxID
	return nil
}

type mailboxMutationBackend struct {
	fakeBackend
	created      MailboxID
	renamedFrom  MailboxID
	renamedTo    MailboxID
	subscribed   MailboxID
	unsubscribed MailboxID
}

func (b *mailboxMutationBackend) CreateMailbox(_ context.Context, _ UserID, mailboxID MailboxID) (Mailbox, error) {
	b.created = mailboxID
	return Mailbox{ID: mailboxID, Name: string(mailboxID), UIDValidity: 1, UIDNext: 1}, nil
}

func (b *mailboxMutationBackend) GetMailbox(_ context.Context, _ UserID, mailboxID MailboxID) (Mailbox, error) {
	return Mailbox{ID: mailboxID, Name: string(mailboxID), UIDValidity: 1, UIDNext: 1}, nil
}

func (b *mailboxMutationBackend) RenameMailbox(_ context.Context, _ UserID, source MailboxID, dest MailboxID) (Mailbox, error) {
	b.renamedFrom = source
	b.renamedTo = dest
	return Mailbox{ID: dest, Name: string(dest), UIDValidity: 1, UIDNext: 1}, nil
}

func (b *mailboxMutationBackend) SubscribeMailbox(_ context.Context, _ UserID, mailboxID MailboxID) (MailboxSubscription, error) {
	b.subscribed = mailboxID
	return MailboxSubscription{Name: string(mailboxID), Mailbox: Mailbox{ID: mailboxID, Name: string(mailboxID)}, Exists: true}, nil
}

func (b *mailboxMutationBackend) UnsubscribeMailbox(_ context.Context, _ UserID, mailboxID MailboxID) error {
	b.unsubscribed = mailboxID
	return nil
}

type operationalMailboxNameBackend struct {
	fakeBackend
	selected     MailboxID
	statusLookup MailboxID
	appended     MailboxID
	appendBody   string
	copyDest     MailboxID
	moveDest     MailboxID
	renameSource MailboxID
	renameDest   MailboxID
}

func (b *operationalMailboxNameBackend) SelectMailbox(_ context.Context, req SelectMailboxRequest) (MailboxState, error) {
	b.selected = req.MailboxID
	return MailboxState{
		Mailbox:        Mailbox{ID: "taipei", Name: "台北", UIDValidity: 10, UIDNext: 12, Messages: 2},
		PermanentFlags: []string{FlagSeen, FlagFlagged, FlagAnswered, FlagDraft, FlagDeleted},
	}, nil
}

func (b *operationalMailboxNameBackend) GetMailbox(_ context.Context, _ UserID, mailboxID MailboxID) (Mailbox, error) {
	switch mailboxID {
	case "台北":
		b.statusLookup = mailboxID
		return Mailbox{ID: "taipei", Name: "台北", UIDValidity: 10, UIDNext: 12, Messages: 2}, nil
	case "日本語":
		return Mailbox{ID: "nihon", Name: "日本語", UIDValidity: 20, UIDNext: 50}, nil
	default:
		return Mailbox{}, ErrMailboxNotFound
	}
}

func (b *operationalMailboxNameBackend) AppendMessage(_ context.Context, req AppendMessageRequest) (AppendMessageResult, error) {
	b.appended = req.MailboxID
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		b.appendBody = string(data)
	}
	return AppendMessageResult{
		Summary:     MessageSummary{ID: "append-44", MailboxID: "taipei", UID: 44},
		UIDValidity: 10,
	}, nil
}

func (b *operationalMailboxNameBackend) CopyMessages(_ context.Context, req CopyMessagesRequest) ([]CopyMessageResult, error) {
	b.copyDest = req.DestMailboxID
	return []CopyMessageResult{{SourceUID: req.UIDs[0], Destination: MessageSummary{ID: "copy-50", MailboxID: req.DestMailboxID, UID: 50}}}, nil
}

func (b *operationalMailboxNameBackend) MoveMessages(_ context.Context, req MoveMessagesRequest) ([]MoveMessageResult, error) {
	b.moveDest = req.DestMailboxID
	return []MoveMessageResult{{
		Source:              MessageSummary{ID: "message-7", MailboxID: req.SourceMailboxID, UID: 7, SequenceNumber: 1},
		Destination:         MessageSummary{ID: "move-51", MailboxID: req.DestMailboxID, UID: 51, SequenceNumber: 1},
		SourceHighestModSeq: 30,
	}}, nil
}

func (b *operationalMailboxNameBackend) RenameMailbox(_ context.Context, _ UserID, source MailboxID, dest MailboxID) (Mailbox, error) {
	b.renameSource = source
	b.renameDest = dest
	return Mailbox{ID: dest, Name: string(dest), UIDValidity: 30, UIDNext: 1}, nil
}

func (childMailboxBackend) ListMailboxes(context.Context, ListMailboxesRequest) ([]Mailbox, error) {
	return []Mailbox{
		{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2},
		{ID: "projects", FullPath: "Projects", UIDValidity: 2, UIDNext: 1},
		{ID: "projects-2026", ParentID: "projects", FullPath: "Projects/2026", UIDValidity: 3, UIDNext: 1},
	}, nil
}

func (nestedMailboxBackend) ListMailboxes(context.Context, ListMailboxesRequest) ([]Mailbox, error) {
	return []Mailbox{
		{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2},
		{ID: "projects", FullPath: "Projects", UIDValidity: 2, UIDNext: 1},
		{ID: "projects-2026", FullPath: "Projects/2026", UIDValidity: 3, UIDNext: 1},
		{ID: "projects-2026-jan", FullPath: "Projects/2026/Jan", UIDValidity: 4, UIDNext: 1},
	}, nil
}

func (specialUseBackend) ListMailboxes(context.Context, ListMailboxesRequest) ([]Mailbox, error) {
	return []Mailbox{
		{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2},
		{ID: "drafts", Name: "Drafts", SystemType: "drafts", UIDValidity: 2, UIDNext: 1},
		{ID: "sent", Name: "Sent", SystemType: "sent", UIDValidity: 3, UIDNext: 1},
		{ID: "trash", Name: "Trash", SystemType: "trash", UIDValidity: 4, UIDNext: 1},
	}, nil
}

func (listStatusBackend) ListMailboxes(context.Context, ListMailboxesRequest) ([]Mailbox, error) {
	return []Mailbox{
		{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 41, Messages: 17, Unseen: 3, HighestModSeq: 70, Size: 4096},
		{ID: "sent", Name: "Sent", SystemType: "sent", UIDValidity: 2, UIDNext: 8, Messages: 5, HighestModSeq: 12, Size: 2048},
	}, nil
}

func (spacedListPatternBackend) ListMailboxes(context.Context, ListMailboxesRequest) ([]Mailbox, error) {
	return []Mailbox{
		{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 41, Messages: 17},
		{ID: "archive-2026", Name: "Archive 2026", UIDValidity: 2, UIDNext: 8, Messages: 9},
	}, nil
}

func (fakeBackend) ListMailboxes(context.Context, ListMailboxesRequest) ([]Mailbox, error) {
	return []Mailbox{
		{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2},
		{ID: "archive", FullPath: "Archive\r\n2026", UIDValidity: 2, UIDNext: 3},
	}, nil
}

func (fakeBackend) ListSubscribedMailboxes(context.Context, ListMailboxesRequest) ([]MailboxSubscription, error) {
	return []MailboxSubscription{
		{Name: "INBOX", Mailbox: Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2}, Exists: true},
	}, nil
}

func (fakeBackend) GetMailbox(_ context.Context, _ UserID, mailboxID MailboxID) (Mailbox, error) {
	switch strings.ToLower(strings.TrimSpace(string(mailboxID))) {
	case "archive":
		return Mailbox{ID: "archive", Name: "Archive", UIDValidity: 2, UIDNext: 3, Size: 64}, nil
	default:
		return Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2, Unseen: 1, Size: 53}, nil
	}
}

func (fakeBackend) SubscribeMailbox(context.Context, UserID, MailboxID) (MailboxSubscription, error) {
	return MailboxSubscription{Name: "INBOX", Mailbox: Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 2}, Exists: true}, nil
}

func (fakeBackend) UnsubscribeMailbox(context.Context, UserID, MailboxID) error {
	return nil
}

func (fakeBackend) CreateMailbox(context.Context, UserID, MailboxID) (Mailbox, error) {
	return Mailbox{ID: "projects", Name: "Projects", UIDValidity: 2, UIDNext: 1}, nil
}

func (fakeBackend) DeleteMailbox(context.Context, UserID, MailboxID) error {
	return nil
}

func (fakeBackend) RenameMailbox(context.Context, UserID, MailboxID, MailboxID) (Mailbox, error) {
	return Mailbox{ID: "archive", Name: "Archive", UIDValidity: 2, UIDNext: 1}, nil
}

func (fakeBackend) ListMessages(context.Context, ListMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{
		{ID: "message-1", UID: 7, SequenceNumber: 1, InternalDate: time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC), Envelope: Envelope{Subject: "Hello IMAP", From: []Address{{Name: "Sender", Mailbox: "sender", Host: "example.net"}}, To: []Address{{Name: "Target User", Mailbox: "target", Host: "example.com"}}, Date: time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)}, Flags: MessageFlags{Read: true, Starred: true}, Size: 11, ModSeq: 17},
		{ID: "message-2", UID: 8, SequenceNumber: 2, InternalDate: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC), Envelope: Envelope{Subject: "Archive", From: []Address{{Name: "Archive Bot", Mailbox: "archive", Host: "example.net"}}, Cc: []Address{{Name: "Review Desk", Mailbox: "review", Host: "example.com"}}, Bcc: []Address{{Name: "Hidden Desk", Mailbox: "hidden", Host: "example.com"}}, Date: time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)}, Flags: MessageFlags{Draft: true}, Size: 42, ModSeq: 23},
	}, nil
}

type flagSearchBackend struct {
	fakeBackend
}

func (flagSearchBackend) ListMessages(context.Context, ListMessagesRequest) ([]MessageSummary, error) {
	messages, _ := (fakeBackend{}).ListMessages(context.Background(), ListMessagesRequest{})
	messages[0].Flags.Forwarded = true
	return messages, nil
}

type recentSearchBackend struct {
	fakeBackend
}

func (recentSearchBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox:        Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 10, Messages: 3, Recent: 2},
		PermanentFlags: []string{FlagSeen, FlagFlagged, FlagAnswered, FlagDraft, FlagDeleted},
	}, nil
}

func (recentSearchBackend) ListMessages(context.Context, ListMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{
		{ID: "message-7", UID: 7, SequenceNumber: 1, Recent: true, Flags: MessageFlags{Read: true}},
		{ID: "message-8", UID: 8, SequenceNumber: 2, Recent: true},
		{ID: "message-9", UID: 9, SequenceNumber: 3},
	}, nil
}

type customKeywordBackend struct {
	fakeBackend

	storeCalls   int
	lastKeywords []string
}

func (customKeywordBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox:        Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 9, Messages: 2},
		PermanentFlags: []string{FlagSeen, "$Project", "$Project", `\Bogus`},
	}, nil
}

func (customKeywordBackend) ListMessages(context.Context, ListMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{
		{ID: "message-7", UID: 7, SequenceNumber: 1, Flags: MessageFlags{Keywords: []string{"$Project"}}},
		{ID: "message-8", UID: 8, SequenceNumber: 2},
	}, nil
}

func (b *customKeywordBackend) StoreFlags(_ context.Context, req StoreFlagsRequest) ([]MessageSummary, error) {
	b.storeCalls++
	b.lastKeywords = append([]string(nil), req.Flags.Keywords...)
	return []MessageSummary{{
		ID:             "message-8",
		UID:            req.UIDs[0],
		SequenceNumber: 2,
		Flags:          req.Flags,
	}}, nil
}

type threadBackend struct {
	fakeBackend
}

func (threadBackend) GetMailbox(_ context.Context, _ UserID, _ MailboxID) (Mailbox, error) {
	return Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 15, Messages: 4, Unseen: 4}, nil
}

func (threadBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox:        Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 15, Messages: 4},
		PermanentFlags: []string{FlagSeen, FlagFlagged, FlagAnswered, FlagDraft, FlagDeleted},
	}, nil
}

func (threadBackend) ListMessages(context.Context, ListMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{
		{ID: "thread-1", UID: 11, SequenceNumber: 1, InternalDate: time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC), Envelope: Envelope{Subject: "Alpha", Date: time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)}, Size: 10},
		{ID: "thread-2", UID: 12, SequenceNumber: 2, InternalDate: time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC), Envelope: Envelope{Subject: "Project", Date: time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC)}, Size: 20},
		{ID: "thread-3", UID: 13, SequenceNumber: 3, InternalDate: time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC), Envelope: Envelope{Subject: "Re: Project", Date: time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)}, Size: 30},
		{ID: "thread-4", UID: 14, SequenceNumber: 4, InternalDate: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC), Envelope: Envelope{Subject: "[fwd: Project]", Date: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)}, Size: 40},
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
	if req.UID == 12 {
		body = testNestedMultipartBody()
		size = int64(len(body))
	}
	if req.UID == 13 {
		body = testMessageRFC822Body()
		size = int64(len(body))
	}
	if req.UID == 14 {
		body = testMultipartMessageRFC822Body()
		size = int64(len(body))
	}
	if req.UID == 15 {
		body = testMessageRFC822NestedMultipartBody()
		size = int64(len(body))
	}
	if req.UID == 16 {
		body = testMalformedMessageRFC822Body()
		size = int64(len(body))
	}
	if req.UID == 17 {
		body = testMultipartMessageRFC822NestedMultipartBody()
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
			ModSeq:       uint64(req.UID + 10),
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}, nil
}

type searchSaveFailureBackend struct {
	fakeBackend
}

func (b searchSaveFailureBackend) FetchMessage(ctx context.Context, req FetchMessageRequest) (Message, error) {
	if req.UID == 7 {
		return Message{}, errors.New("search body fetch failed")
	}
	return b.fakeBackend.FetchMessage(ctx, req)
}

type fetchFailureBackend struct {
	fakeBackend
}

func (fetchFailureBackend) FetchMessage(context.Context, FetchMessageRequest) (Message, error) {
	return Message{}, errors.New("fetch failed")
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

func testNestedMultipartBody() string {
	return strings.Join([]string{
		"Content-Type: multipart/mixed; boundary=outer",
		"",
		"--outer",
		"Content-Type: multipart/alternative; boundary=alt",
		"",
		"--alt",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"plain",
		"--alt",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<b>html</b>",
		"--alt--",
		"--outer--",
		"",
	}, "\r\n")
}

func testMessageRFC822Body() string {
	return strings.Join([]string{
		"Content-Type: message/rfc822",
		"Content-Transfer-Encoding: 7bit",
		"",
		"Subject: Nested",
		"From: nested@example.net",
		"",
		"nested body",
	}, "\r\n")
}

func testMultipartMessageRFC822Body() string {
	return strings.Join([]string{
		"Content-Type: multipart/mixed; boundary=forwarded",
		"",
		"--forwarded",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"see attached",
		"--forwarded",
		"Content-Type: message/rfc822",
		"",
		"Subject: Attached",
		"From: attached@example.net",
		"",
		"attached body",
		"--forwarded--",
		"",
	}, "\r\n")
}

func testMessageRFC822NestedMultipartBody() string {
	return strings.Join([]string{
		"Content-Type: message/rfc822",
		"",
		"Subject: Nested Multipart",
		"Content-Type: multipart/alternative; boundary=nested-alt",
		"",
		"--nested-alt",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"plain",
		"--nested-alt",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<b>html</b>",
		"--nested-alt--",
		"",
	}, "\r\n")
}

func testMultipartMessageRFC822NestedMultipartBody() string {
	return strings.Join([]string{
		"Content-Type: multipart/mixed; boundary=wrap",
		"",
		"--wrap",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"see forwarded multipart",
		"--wrap",
		"Content-Type: message/rfc822",
		"",
		testAttachedNestedMultipartMessage(),
		"--wrap--",
		"",
	}, "\r\n")
}

func testAttachedNestedMultipartMessage() string {
	return strings.Join([]string{
		"Subject: Attached Multipart",
		"Content-Type: multipart/alternative; boundary=attached-alt",
		"",
		"--attached-alt",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"plain attached",
		"--attached-alt",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<strong>html</strong>",
		"--attached-alt--",
	}, "\r\n")
}

func imapTestLineCount(value string) int {
	if value == "" {
		return 0
	}
	lines := strings.Count(value, "\n")
	if !strings.HasSuffix(value, "\n") {
		lines++
	}
	return lines
}

func testMalformedMessageRFC822Body() string {
	return strings.Join([]string{
		"Content-Type: message/rfc822",
		"",
		"not a header line",
		"still raw",
	}, "\r\n")
}

func (fakeBackend) StoreFlags(_ context.Context, req StoreFlagsRequest) ([]MessageSummary, error) {
	summaries := make([]MessageSummary, 0, len(req.UIDs))
	modified := make([]UID, 0)
	for _, uid := range req.UIDs {
		modseq := uint64(uid + 20)
		if req.UnchangedSinceSet && modseq > req.UnchangedSince {
			modified = append(modified, uid)
			continue
		}
		summaries = append(summaries, MessageSummary{ID: MessageID(fmt.Sprintf("message-%d", uid)), UID: uid, SequenceNumber: uint32(uid - 6), Flags: MessageFlags{Read: req.Flags.Read, Starred: req.Flags.Starred, Answered: req.Flags.Answered, Forwarded: req.Flags.Forwarded, Draft: req.Flags.Draft, Deleted: req.Flags.Deleted}, ModSeq: modseq})
	}
	if len(modified) > 0 {
		return summaries, &StoreModifiedError{UIDs: modified, Summaries: summaries}
	}
	return summaries, nil
}

type forwardedPermanentFlagsBackend struct {
	fakeBackend
}

func (forwardedPermanentFlagsBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox:        Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2},
		PermanentFlags: []string{FlagSeen, FlagFlagged, FlagAnswered, FlagForwarded, FlagDraft, FlagDeleted},
	}, nil
}

type duplicatePermanentFlagsBackend struct {
	fakeBackend
}

func (duplicatePermanentFlagsBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox: Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2},
		PermanentFlags: []string{
			`\seen`,
			FlagSeen,
			`Forwarded`,
			FlagForwarded,
			`\Bogus`,
			FlagDeleted,
			FlagDeleted,
		},
	}, nil
}

type limitedPermanentFlagsBackend struct {
	fakeBackend
	storeCalls int
}

func (limitedPermanentFlagsBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox:        Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2},
		PermanentFlags: []string{FlagSeen},
	}, nil
}

func (b *limitedPermanentFlagsBackend) StoreFlags(_ context.Context, req StoreFlagsRequest) ([]MessageSummary, error) {
	b.storeCalls++
	return []MessageSummary{{ID: "message-7", UID: req.UIDs[0], SequenceNumber: 1, Flags: req.Flags}}, nil
}

type noPermanentFlagsBackend struct {
	fakeBackend
	storeCalls int
}

func (noPermanentFlagsBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox:        Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2},
		PermanentFlags: nil,
	}, nil
}

func (b *noPermanentFlagsBackend) StoreFlags(context.Context, StoreFlagsRequest) ([]MessageSummary, error) {
	b.storeCalls++
	return nil, nil
}

type emptyFlagStoreBackend struct {
	fakeBackend

	calls     int
	lastMode  StoreFlagsMode
	lastFlags MessageFlags
}

func (b *emptyFlagStoreBackend) StoreFlags(_ context.Context, req StoreFlagsRequest) ([]MessageSummary, error) {
	b.calls++
	b.lastMode = req.Mode
	b.lastFlags = req.Flags
	return []MessageSummary{{
		ID:             "message-7",
		UID:            7,
		SequenceNumber: 1,
		Flags:          req.Flags,
		ModSeq:         27,
	}}, nil
}

type staleStoreBackend struct {
	fakeBackend
}

func (staleStoreBackend) StoreFlags(_ context.Context, req StoreFlagsRequest) ([]MessageSummary, error) {
	summaries := []MessageSummary{
		{ID: "message-7", UID: 7, SequenceNumber: 1, Flags: MessageFlags{Read: req.Flags.Read}, ModSeq: 27},
		{ID: "message-8", UID: 8, SequenceNumber: 2, Flags: MessageFlags{Read: false}, ModSeq: 29},
	}
	return summaries, &StoreModifiedError{UIDs: []UID{8}, Summaries: summaries}
}

type bodyFetchSeenBackend struct {
	fakeBackend

	mu    sync.Mutex
	seen  bool
	calls int
}

func (b *bodyFetchSeenBackend) FetchMessage(_ context.Context, req FetchMessageRequest) (Message, error) {
	b.mu.Lock()
	seen := b.seen
	b.mu.Unlock()
	return Message{
		Summary: MessageSummary{
			ID:             "message-7",
			UID:            req.UID,
			SequenceNumber: 1,
			Flags:          MessageFlags{Read: seen},
			Size:           11,
			ModSeq:         17,
		},
		Body: io.NopCloser(strings.NewReader("hello world")),
	}, nil
}

func (b *bodyFetchSeenBackend) StoreFlags(_ context.Context, req StoreFlagsRequest) ([]MessageSummary, error) {
	if req.Mode != StoreFlagsAdd || !req.Flags.Read || len(req.UIDs) != 1 || req.UIDs[0] != 7 {
		return nil, fmt.Errorf("unexpected store flags request: %+v", req)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seen = true
	b.calls++
	return []MessageSummary{{
		ID:             "message-7",
		UID:            7,
		SequenceNumber: 1,
		Flags:          MessageFlags{Read: true},
		Size:           11,
		ModSeq:         18,
	}}, nil
}

func (b *bodyFetchSeenBackend) storeCalls() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

func (fakeBackend) AppendMessage(context.Context, AppendMessageRequest) (AppendMessageResult, error) {
	return AppendMessageResult{}, ErrUnsupportedAppend
}

type appendBackend struct {
	fakeBackend
	request AppendMessageRequest
	body    string
	result  AppendMessageResult
	err     error
}

func (b *appendBackend) AppendMessage(_ context.Context, req AppendMessageRequest) (AppendMessageResult, error) {
	b.request = req
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		b.body = string(data)
	}
	if b.err != nil {
		return AppendMessageResult{}, b.err
	}
	return b.result, nil
}

type appendEventBackend struct {
	appendBackend
	events chan MailboxEvent
}

func (b *appendEventBackend) Subscribe(context.Context, UserID, MailboxID) (<-chan MailboxEvent, func(), error) {
	cancel := func() {}
	return b.events, cancel, nil
}

func (fakeBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox:        Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2},
		PermanentFlags: []string{FlagSeen, FlagFlagged, FlagAnswered, FlagDraft, FlagDeleted},
	}, nil
}

type unseenSelectBackend struct {
	fakeBackend
}

func (unseenSelectBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox:        Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2, Unseen: 1},
		PermanentFlags: []string{FlagSeen, FlagFlagged, FlagAnswered, FlagDraft, FlagDeleted},
	}, nil
}

type modSeqBackend struct {
	fakeBackend
}

func (modSeqBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox:        Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, HighestModSeq: 9, Messages: 2},
		PermanentFlags: []string{FlagSeen, FlagFlagged, FlagAnswered, FlagDraft, FlagDeleted},
	}, nil
}

func (modSeqBackend) GetMailbox(context.Context, UserID, MailboxID) (Mailbox, error) {
	return Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, HighestModSeq: 9, Messages: 2}, nil
}

type uidNotStickyBackend struct {
	fakeBackend
}

func (uidNotStickyBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{
		Mailbox:        Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2},
		PermanentFlags: []string{FlagSeen, FlagFlagged, FlagAnswered, FlagDraft, FlagDeleted},
		UIDNotSticky:   true,
	}, nil
}

func (fakeBackend) CopyMessages(context.Context, CopyMessagesRequest) ([]CopyMessageResult, error) {
	return []CopyMessageResult{{SourceUID: 7, Destination: MessageSummary{ID: "message-copy-1", MailboxID: "inbox", UID: 9}}}, nil
}

type copyBackend struct {
	fakeBackend
	requests []CopyMessagesRequest
	nextUID  UID
}

func (b *copyBackend) GetMailbox(_ context.Context, _ UserID, mailboxID MailboxID) (Mailbox, error) {
	switch strings.ToLower(strings.TrimSpace(string(mailboxID))) {
	case "archive":
		return Mailbox{ID: "archive", Name: "Archive", UIDValidity: 2, UIDNext: 3}, nil
	default:
		return Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2, Unseen: 1}, nil
	}
}

func (b *copyBackend) CopyMessages(_ context.Context, req CopyMessagesRequest) ([]CopyMessageResult, error) {
	b.requests = append(b.requests, req)
	if b.nextUID == 0 {
		b.nextUID = 9
	}
	results := make([]CopyMessageResult, 0, len(req.UIDs))
	for _, sourceUID := range req.UIDs {
		results = append(results, CopyMessageResult{SourceUID: sourceUID, Destination: MessageSummary{ID: MessageID(fmt.Sprintf("message-copy-%d", b.nextUID)), MailboxID: req.DestMailboxID, UID: b.nextUID}})
		b.nextUID++
	}
	return results, nil
}

type quotedMailboxTransferBackend struct {
	fakeBackend
	mailboxLookups []MailboxID
	copyRequests   []CopyMessagesRequest
	moveRequests   []MoveMessagesRequest
	nextCopyUID    UID
}

func (b *quotedMailboxTransferBackend) GetMailbox(_ context.Context, _ UserID, mailboxID MailboxID) (Mailbox, error) {
	b.mailboxLookups = append(b.mailboxLookups, mailboxID)
	if mailboxID == "Team Archive" {
		return Mailbox{ID: "team-archive", Name: "Team Archive", UIDValidity: 7, UIDNext: 20}, nil
	}
	if mailboxID == `Team "Archive"` {
		return Mailbox{ID: "team-quote-archive", Name: `Team "Archive"`, UIDValidity: 8, UIDNext: 20}, nil
	}
	return Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2, Unseen: 1}, nil
}

func (b *quotedMailboxTransferBackend) CopyMessages(_ context.Context, req CopyMessagesRequest) ([]CopyMessageResult, error) {
	b.copyRequests = append(b.copyRequests, req)
	if b.nextCopyUID == 0 {
		b.nextCopyUID = 20
	}
	result := CopyMessageResult{
		SourceUID:   req.UIDs[0],
		Destination: MessageSummary{ID: MessageID(fmt.Sprintf("message-copy-%d", b.nextCopyUID)), MailboxID: req.DestMailboxID, UID: b.nextCopyUID},
	}
	b.nextCopyUID++
	return []CopyMessageResult{result}, nil
}

func (b *quotedMailboxTransferBackend) MoveMessages(_ context.Context, req MoveMessagesRequest) ([]MoveMessageResult, error) {
	b.moveRequests = append(b.moveRequests, req)
	return []MoveMessageResult{{
		Source:              MessageSummary{ID: "message-7", MailboxID: req.SourceMailboxID, UID: 7, SequenceNumber: 1},
		Destination:         MessageSummary{ID: "message-move-30", MailboxID: req.DestMailboxID, UID: 30, SequenceNumber: 1},
		SourceHighestModSeq: 19,
	}}, nil
}

type selectedCopyBackend struct {
	fakeBackend
}

func (selectedCopyBackend) CopyMessages(_ context.Context, req CopyMessagesRequest) ([]CopyMessageResult, error) {
	return []CopyMessageResult{{
		SourceUID: req.UIDs[0],
		Destination: MessageSummary{
			ID:             "message-copy-11",
			MailboxID:      req.DestMailboxID,
			UID:            11,
			SequenceNumber: 5,
		},
	}}, nil
}

type missingDestinationBackend struct {
	fakeBackend
}

func (missingDestinationBackend) GetMailbox(_ context.Context, _ UserID, mailboxID MailboxID) (Mailbox, error) {
	if strings.EqualFold(strings.TrimSpace(string(mailboxID)), "missing") {
		return Mailbox{}, ErrMailboxNotFound
	}
	return Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2, Unseen: 1}, nil
}

type uidNotStickyDestinationBackend struct {
	fakeBackend
}

func (uidNotStickyDestinationBackend) GetMailbox(_ context.Context, _ UserID, mailboxID MailboxID) (Mailbox, error) {
	if strings.EqualFold(strings.TrimSpace(string(mailboxID)), "archive") {
		return Mailbox{ID: "archive", Name: "Archive", UIDValidity: 2, UIDNext: 3, UIDNotSticky: true}, nil
	}
	return Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 5, Messages: 2, Unseen: 1}, nil
}

type sameMailboxMoveBackend struct {
	fakeBackend
}

func (sameMailboxMoveBackend) MoveMessages(context.Context, MoveMessagesRequest) ([]MoveMessageResult, error) {
	return []MoveMessageResult{{
		Source:              MessageSummary{ID: "message-7", MailboxID: "inbox", UID: 7, SequenceNumber: 1},
		Destination:         MessageSummary{ID: "message-copy-9", MailboxID: "inbox", UID: 9, SequenceNumber: 3},
		SourceHighestModSeq: 19,
	}}, nil
}

func (fakeBackend) MoveMessages(context.Context, MoveMessagesRequest) ([]MoveMessageResult, error) {
	return []MoveMessageResult{{
		Source:              MessageSummary{ID: "message-7", MailboxID: "inbox", UID: 7, SequenceNumber: 1},
		Destination:         MessageSummary{ID: "message-7", MailboxID: "archive", UID: 9, SequenceNumber: 1},
		SourceHighestModSeq: 19,
	}}, nil
}

func (fakeBackend) Expunge(context.Context, ExpungeRequest) ([]MessageSummary, error) {
	return []MessageSummary{{ID: "message-7", MailboxID: "inbox", UID: 7, SequenceNumber: 1, Flags: MessageFlags{Deleted: true}}}, nil
}

type closeBackend struct {
	fakeBackend
	expungeCount     int
	expungeUserID    UserID
	expungeMailboxID MailboxID
}

func (b *closeBackend) Expunge(_ context.Context, req ExpungeRequest) ([]MessageSummary, error) {
	b.expungeCount++
	b.expungeUserID = req.UserID
	b.expungeMailboxID = req.MailboxID
	return []MessageSummary{{ID: "message-7", MailboxID: req.MailboxID, UID: 7, SequenceNumber: 1, Flags: MessageFlags{Deleted: true}}}, nil
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

type eventSequenceBackend struct {
	eventBackend
}

func (b *eventSequenceBackend) ListMessages(context.Context, ListMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{
		{ID: "message-1", UID: 7, SequenceNumber: 1, Flags: MessageFlags{Read: true, Starred: true}, Size: 11},
		{ID: "message-2", UID: 8, SequenceNumber: 2, Flags: MessageFlags{Draft: true}, Size: 42},
		{ID: "message-3", UID: 9, SequenceNumber: 3, Flags: MessageFlags{Answered: true}, Size: 9},
	}, nil
}

func (b *eventSequenceBackend) FetchMessage(_ context.Context, req FetchMessageRequest) (Message, error) {
	if req.UID != 9 {
		return (fakeBackend{}).FetchMessage(context.Background(), req)
	}
	return Message{
		Summary: MessageSummary{
			ID:             "message-3",
			UID:            9,
			SequenceNumber: 3,
			Flags:          MessageFlags{Answered: true},
			Size:           9,
		},
		Body: io.NopCloser(strings.NewReader("answered\n")),
	}, nil
}

type literalLoginBackend struct {
	fakeBackend
	creds chan [2]string
}

func (b literalLoginBackend) Authenticate(_ context.Context, username, password string) (Session, error) {
	b.creds <- [2]string{username, password}
	return Session{UserID: "user-1", Username: username}, nil
}

type canonicalMailboxBackend struct {
	fakeBackend
	selectMailboxID    MailboxID
	subscribeMailboxID MailboxID
}

func (b *canonicalMailboxBackend) SelectMailbox(_ context.Context, req SelectMailboxRequest) (MailboxState, error) {
	b.selectMailboxID = req.MailboxID
	return MailboxState{
		Mailbox:        Mailbox{ID: "mailbox-uuid", Name: "INBOX", UIDValidity: 1, UIDNext: 1},
		PermanentFlags: []string{FlagSeen, FlagFlagged, FlagAnswered, FlagDraft},
	}, nil
}

func (b *canonicalMailboxBackend) Subscribe(_ context.Context, _ UserID, mailboxID MailboxID) (<-chan MailboxEvent, func(), error) {
	b.subscribeMailboxID = mailboxID
	events := make(chan MailboxEvent)
	cancel := func() { close(events) }
	return events, cancel, nil
}

type selectModeBackend struct {
	fakeBackend
	readOnly bool
}

func (b *selectModeBackend) SelectMailbox(ctx context.Context, req SelectMailboxRequest) (MailboxState, error) {
	b.readOnly = req.ReadOnly
	return b.fakeBackend.SelectMailbox(ctx, req)
}

type selectMissingAfterSelectBackend struct {
	fakeBackend
	canceled int
}

func (b *selectMissingAfterSelectBackend) SelectMailbox(ctx context.Context, req SelectMailboxRequest) (MailboxState, error) {
	if req.MailboxID == "missing" {
		return MailboxState{}, ErrMailboxNotFound
	}
	return b.fakeBackend.SelectMailbox(ctx, req)
}

func (b *selectMissingAfterSelectBackend) Subscribe(context.Context, UserID, MailboxID) (<-chan MailboxEvent, func(), error) {
	events := make(chan MailboxEvent)
	cancel := func() {
		b.canceled++
		close(events)
	}
	return events, cancel, nil
}

type failingSubscribeBackend struct {
	fakeBackend
}

func (failingSubscribeBackend) Subscribe(context.Context, UserID, MailboxID) (<-chan MailboxEvent, func(), error) {
	return nil, nil, errors.New("subscription unavailable")
}

type selectSubscribeCancelBackend struct {
	fakeBackend
	canceled bool
}

func (b *selectSubscribeCancelBackend) Subscribe(context.Context, UserID, MailboxID) (<-chan MailboxEvent, func(), error) {
	events := make(chan MailboxEvent)
	cancel := func() {
		b.canceled = true
		close(events)
	}
	return events, cancel, nil
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

type renameSelectedBackend struct {
	fakeBackend
	renamedFrom          MailboxID
	renamedTo            MailboxID
	subscribed           MailboxID
	renamedHighestModSeq uint64
}

func (renameSelectedBackend) GetMailbox(_ context.Context, _ UserID, mailboxID MailboxID) (Mailbox, error) {
	if strings.EqualFold(strings.TrimSpace(string(mailboxID)), "Archive") {
		return Mailbox{ID: "archive", Name: "Archive", UIDValidity: 2, UIDNext: 5, HighestModSeq: 10}, nil
	}
	return Mailbox{}, ErrMailboxNotFound
}

func (b *renameSelectedBackend) RenameMailbox(_ context.Context, _ UserID, source MailboxID, dest MailboxID) (Mailbox, error) {
	b.renamedFrom = source
	b.renamedTo = dest
	return Mailbox{ID: "renamed", Name: string(dest), UIDValidity: 2, UIDNext: 5, HighestModSeq: b.renamedHighestModSeq}, nil
}

func (b *renameSelectedBackend) Subscribe(_ context.Context, _ UserID, mailboxID MailboxID) (<-chan MailboxEvent, func(), error) {
	b.subscribed = mailboxID
	events := make(chan MailboxEvent)
	cancel := func() { close(events) }
	return events, cancel, nil
}

type missingMailboxBackend struct {
	fakeBackend
}

type selectMissingBackend struct {
	fakeBackend
}

func (selectMissingBackend) SelectMailbox(ctx context.Context, req SelectMailboxRequest) (MailboxState, error) {
	if req.MailboxID == "missing" {
		return MailboxState{}, ErrMailboxNotFound
	}
	return fakeBackend{}.SelectMailbox(ctx, req)
}

func (missingMailboxBackend) SelectMailbox(context.Context, SelectMailboxRequest) (MailboxState, error) {
	return MailboxState{}, ErrMailboxNotFound
}

func (missingMailboxBackend) GetMailbox(context.Context, UserID, MailboxID) (Mailbox, error) {
	return Mailbox{}, ErrMailboxNotFound
}

func (missingMailboxBackend) DeleteMailbox(context.Context, UserID, MailboxID) error {
	return ErrMailboxNotFound
}

func (missingMailboxBackend) RenameMailbox(context.Context, UserID, MailboxID, MailboxID) (Mailbox, error) {
	return Mailbox{}, ErrMailboxNotFound
}
