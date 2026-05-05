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
	"strings"
	"sync"
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
	if line != "* CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT SASL-IR AUTH=PLAIN\r\n" {
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
		"* BAD malformed command\r\n",
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
	if _, err := client.Write([]byte("a1 LOGIN \"user\"secret pass\r\na2 LOGIN \"user\\n\" pass\r\na3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write malformed quoted arguments: %v", err)
	}
	want := []string{
		"* BAD malformed command\r\n",
		"* BAD malformed command\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a3 OK LOGOUT completed\r\n",
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
	if _, err := client.Write([]byte("a1 CAPABILITY)\r\na2 LOGIN user@example.com secret\r\na3 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write malformed command atom setup: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 BAD malformed command\r\n" {
		t.Fatalf("malformed command response = %q err = %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a2 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a4 UID FETCH] 7 (FLAGS)\r\na5 LOGOUT\r\n")); err != nil {
		t.Fatalf("write malformed uid subcommand: %v", err)
	}
	want := []string{
		"a4 BAD malformed command\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a5 OK LOGOUT completed\r\n",
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
		"a1 OK LOGIN completed\r\n",
		"a2 BAD UID command not implemented\r\n",
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
	if _, err := client.Write([]byte("a1 UID\r\na2 UID FETCH]\r\na3 UID BOGUS\r\na4 UID FETCH\r\na5 UID STORE\r\na6 UID EXPUNGE\r\na7 UID COPY 7 &Jjo!\r\na8 UID MOVE 7 &Jjo!\r\na9 UID FETCH 7 (FLAGS)\r\na10 UID SEARCH ALL\r\na11 LOGOUT\r\n")); err != nil {
		t.Fatalf("write uid auth commands: %v", err)
	}
	want := []string{
		"a1 BAD UID command not implemented\r\n",
		"a2 BAD malformed command\r\n",
		"a3 BAD UID command not implemented\r\n",
		"a4 BAD UID FETCH requires UID set and data items\r\n",
		"a5 BAD UID STORE requires UID, mode, and flags\r\n",
		"a6 BAD UID EXPUNGE requires UID set\r\n",
		"a7 BAD UID COPY destination mailbox name is not valid modified UTF-7\r\n",
		"a8 BAD UID MOVE destination mailbox name is not valid modified UTF-7\r\n",
		"a9 NO authentication required\r\n",
		"a10 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a11 OK LOGOUT completed\r\n",
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 FETCH\r\na3 STORE\r\na4 COPY 1\r\na5 COPY 1 &Jjo!\r\na6 MOVE 1\r\na7 SEARCH\r\na8 SEARCH RETURN (COUNT COUNT) ALL\r\na9 SORT\r\na10 SORT (DATE) UTF-8\r\na11 THREAD\r\na12 THREAD REFERENCES UTF-8 ALL\r\na13 FETCH 1 (FLAGS)\r\na14 LOGOUT\r\n")); err != nil {
		t.Fatalf("write selected-state commands: %v", err)
	}
	want := []string{
		"a1 OK LOGIN completed\r\n",
		"a2 BAD FETCH requires sequence set and data items\r\n",
		"a3 BAD STORE requires sequence set, mode, and flags\r\n",
		"a4 BAD COPY requires sequence set and destination mailbox\r\n",
		"a5 BAD COPY destination mailbox name is not valid modified UTF-7\r\n",
		"a6 BAD MOVE requires sequence set and destination mailbox\r\n",
		"a7 BAD SEARCH requires criteria\r\n",
		"a8 BAD SEARCH return options are unsupported\r\n",
		"a9 BAD SORT requires sort criteria, charset, and search criteria\r\n",
		"a10 BAD SORT requires sort criteria, charset, and search criteria\r\n",
		"a11 BAD THREAD requires algorithm, charset, and search criteria\r\n",
		"a12 BAD THREAD algorithm is unsupported\r\n",
		"a13 NO mailbox must be selected\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a14 OK LOGOUT completed\r\n",
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
	if _, err := client.Write([]byte("a1 FETCH\r\na2 STORE\r\na3 COPY 1\r\na4 COPY 1 &Jjo!\r\na5 MOVE 1\r\na6 MOVE 1 &Jjo!\r\na7 FETCH 1 (FLAGS)\r\na8 STORE 1 +FLAGS (\\Seen)\r\na9 COPY 1 Archive\r\na10 MOVE 1 Archive\r\na11 LOGOUT\r\n")); err != nil {
		t.Fatalf("write selected action auth commands: %v", err)
	}
	want := []string{
		"a1 BAD FETCH requires sequence set and data items\r\n",
		"a2 BAD STORE requires sequence set, mode, and flags\r\n",
		"a3 BAD COPY requires sequence set and destination mailbox\r\n",
		"a4 BAD COPY destination mailbox name is not valid modified UTF-7\r\n",
		"a5 BAD MOVE requires sequence set and destination mailbox\r\n",
		"a6 BAD MOVE destination mailbox name is not valid modified UTF-7\r\n",
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
	if _, err := client.Write([]byte("a1 APPEND\r\na2 APPEND inbox BAD\r\na3 APPEND &Jjo! {5+}\r\nhello\r\na4 APPEND inbox BAD {5+}\r\nhello\r\na5 APPEND inbox (\\Seen {5+}\r\nhello\r\na6 APPEND inbox {5+}\r\nhello\r\na7 LOGOUT\r\n")); err != nil {
		t.Fatalf("write append auth commands: %v", err)
	}
	want := []string{
		"a1 BAD APPEND requires mailbox and literal\r\n",
		"a2 BAD APPEND requires mailbox and literal\r\n",
		"a3 BAD APPEND mailbox name is not valid modified UTF-7\r\n",
		"a4 BAD APPEND options are unsupported\r\n",
		"a5 BAD APPEND options are unsupported\r\n",
		"a6 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a7 OK LOGOUT completed\r\n",
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
	if _, err := client.Write([]byte("a1 SEARCH\r\na2 SEARCH RETURN (COUNT COUNT) ALL\r\na3 SORT\r\na4 SORT DATE UTF-8 ALL\r\na5 SORT (DATE) UTF-8\r\na6 THREAD\r\na7 THREAD REFERENCES UTF-8 ALL\r\na8 SEARCH ALL\r\na9 SORT (DATE) UTF-8 ALL\r\na10 THREAD ORDEREDSUBJECT UTF-8 ALL\r\na11 LOGOUT\r\n")); err != nil {
		t.Fatalf("write search auth commands: %v", err)
	}
	want := []string{
		"a1 BAD SEARCH requires criteria\r\n",
		"a2 BAD SEARCH return options are unsupported\r\n",
		"a3 BAD SORT requires sort criteria, charset, and search criteria\r\n",
		"a4 BAD SORT arguments are unsupported\r\n",
		"a5 BAD SORT requires sort criteria, charset, and search criteria\r\n",
		"a6 BAD THREAD requires algorithm, charset, and search criteria\r\n",
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
		"a1 OK LOGIN completed\r\n",
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
	if line, err := reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT STARTTLS LOGINDISABLED\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a3 OK [CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT SASL-IR AUTH=PLAIN] Begin TLS negotiation now\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT SASL-IR AUTH=PLAIN\r\n" {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com\r\na2 AUTHENTICATE BOGUS\r\na3 AUTHENTICATE PLAIN\r\na4 LOGIN user@example.com secret\r\na5 LOGOUT\r\n")); err != nil {
		t.Fatalf("write auth commands: %v", err)
	}
	want := []string{
		"a1 BAD LOGIN requires username and password atoms\r\n",
		"a2 BAD AUTHENTICATE mechanism is unsupported\r\n",
		"a3 NO [PRIVACYREQUIRED] TLS is required for AUTHENTICATE\r\n",
		"a4 NO [PRIVACYREQUIRED] TLS is required for LOGIN\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a5 OK LOGOUT completed\r\n",
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
	if line != "* CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT\r\n" {
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

func TestServerRejectsMalformedIDArguments(t *testing.T) {
	t.Parallel()

	for _, command := range []string{
		`ID`,
		`ID NIL "extra"`,
		`ID "name" "client"`,
		`ID ("name")`,
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
	if line, err = reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	for i := 0; i < 8; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read login/select condstore response: %v", err)
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
		"a1 OK LOGIN completed\r\n",
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
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if _, err := client.Write([]byte("a1 ENABLE\r\na2 ENABLE CONDSTORE\r\na3 LOGOUT\r\n")); err != nil {
		t.Fatalf("write enable auth commands: %v", err)
	}
	want := []string{
		"a1 BAD ENABLE requires at least one capability\r\n",
		"a2 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a3 OK LOGOUT completed\r\n",
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
		"a1 OK LOGIN completed\r\n",
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
		"a1 OK LOGIN completed\r\n",
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
		"a1 OK LOGIN completed\r\n",
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
	if _, err := client.Write([]byte("a7 LOGOUT\r\n")); err != nil {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "* CAPABILITY IMAP4rev1 LITERAL+ IDLE ID NAMESPACE CHILDREN UNSELECT UIDPLUS MOVE CONDSTORE ENABLE SPECIAL-USE LIST-STATUS ESEARCH SEARCHRES STATUS=SIZE SORT THREAD=ORDEREDSUBJECT\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 MOVE 1 Archive\r\na4 UID MOVE 7 Archive\r\na5 APPEND inbox NIL\r\n")); err != nil {
		t.Fatalf("write unsupported mutation commands: %v", err)
	}
	want := []string{
		"* OK [COPYUID 2 7 9] MOVE UID mapping\r\n",
		"* OK [HIGHESTMODSEQ 19] MOVE source mod-sequence\r\n",
		"* 1 EXPUNGE\r\n",
		"a3 OK MOVE completed\r\n",
		"* OK [COPYUID 2 7 9] UID MOVE UID mapping\r\n",
		"* OK [HIGHESTMODSEQ 19] UID MOVE source mod-sequence\r\n",
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
		"* OK [COPYUID 1 7 9] UID MOVE UID mapping\r\n",
		"* 3 EXISTS\r\n",
		"* OK [HIGHESTMODSEQ 19] UID MOVE source mod-sequence\r\n",
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 COPY 1:2 Archive\r\na4 UID COPY 7 Archive\r\n")); err != nil {
		t.Fatalf("write copy commands: %v", err)
	}
	want := []string{
		"a3 OK [COPYUID 2 7,8 9,10] COPY completed\r\n",
		"a4 OK [COPYUID 2 7 11] UID COPY completed\r\n",
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
	if len(backendImpl.requests) != 2 {
		t.Fatalf("copy request count = %d, want 2", len(backendImpl.requests))
	}
	if got, want := backendImpl.requests[0].UIDs, []UID{7, 8}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sequence COPY UIDs = %v, want %v", got, want)
	}
	if got, want := backendImpl.requests[1].UIDs, []UID{7}; !reflect.DeepEqual(got, want) {
		t.Fatalf("UID COPY UIDs = %v, want %v", got, want)
	}
	for _, req := range backendImpl.requests {
		if req.SourceMailboxID != "inbox" || req.DestMailboxID != "archive" || req.UserID != "user-1" {
			t.Fatalf("copy request = %+v, want user-1 inbox -> archive", req)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" *\r\n")); err != nil {
		t.Fatalf("write login/list: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	want := []string{
		"* LIST (\\HasNoChildren) \"/\" \"INBOX\"\r\n",
		"* LIST (\\HasChildren) \"/\" \"Projects\"\r\n",
		"* LIST (\\HasNoChildren) \"/\" \"Projects/2026\"\r\n",
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST (SPECIAL-USE) \"\" *\r\na3 LIST \"\" * RETURN (SPECIAL-USE)\r\na4 LIST (REMOTE) \"\" *\r\n")); err != nil {
		t.Fatalf("write login/list special-use extended: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if _, err := client.Write([]byte("a5 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 LIST \"\" * RETURN (STATUS (MESSAGES UNSEEN UIDNEXT HIGHESTMODSEQ SIZE))\r\na3 LIST \"\" * RETURN (SPECIAL-USE STATUS (MESSAGES SIZE))\r\na4 LIST \"\" * RETURN (STATUS)\r\na5 LIST \"\" * RETURN (STATUS MESSAGES)\r\n")); err != nil {
		t.Fatalf("write list-status: %v", err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
		"a4 BAD LIST requires reference and mailbox pattern atoms\r\n",
		"a5 BAD LIST requires reference and mailbox pattern atoms\r\n",
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
	if _, err := client.Write([]byte("a6 LOGOUT\r\n")); err != nil {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
		"a1 OK LOGIN completed\r\n",
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if _, err := client.Write([]byte("a5 COPY 1 &ZeVnLIqe-\r\na6 UID MOVE 7 &ZeVnLIqe-\r\n")); err != nil {
		t.Fatalf("write copy/move: %v", err)
	}
	want := []string{
		"a5 OK [COPYUID 20 7 50] COPY completed\r\n",
		"* OK [COPYUID 20 7 51] UID MOVE UID mapping\r\n",
		"* OK [HIGHESTMODSEQ 30] UID MOVE source mod-sequence\r\n",
		"* 1 EXPUNGE\r\n",
		"a6 OK UID MOVE completed\r\n",
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
	if _, err := client.Write([]byte("a7 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	if err := <-errCh; err != nil {
		t.Fatalf("ServeConn returned error: %v", err)
	}
	if backendImpl.selected != "台北" || backendImpl.statusLookup != "台北" || backendImpl.appended != "台北" || backendImpl.appendBody != "hello" {
		t.Fatalf("decoded select/status/append = %q/%q/%q body %q", backendImpl.selected, backendImpl.statusLookup, backendImpl.appended, backendImpl.appendBody)
	}
	if backendImpl.copyDest != "nihon" || backendImpl.moveDest != "nihon" {
		t.Fatalf("decoded copy/move destination IDs = %q/%q, want nihon/nihon", backendImpl.copyDest, backendImpl.moveDest)
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
		"a1 OK LOGIN completed\r\n",
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
	if _, err := client.Write([]byte("a1 LIST \"\"\r\na2 LIST \"\" &Jjo!\r\na3 LSUB \"\"\r\na4 CREATE &Jjo!\r\na5 DELETE &Jjo!\r\na6 RENAME Archive\r\na7 RENAME Archive &Jjo!\r\na8 SUBSCRIBE\r\na9 SUBSCRIBE &Jjo!\r\na10 CREATE Projects\r\na11 LOGOUT\r\n")); err != nil {
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
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a11 OK LOGOUT completed\r\n",
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
	if _, err := client.Write([]byte("a1 NAMESPACE extra\r\na2 SELECT\r\na3 SELECT &Jjo!\r\na4 EXAMINE inbox (QRESYNC)\r\na5 STATUS\r\na6 STATUS inbox MESSAGES\r\na7 STATUS inbox (BADITEM)\r\na8 STATUS &Jjo! (MESSAGES)\r\na9 SELECT inbox\r\na10 STATUS inbox (MESSAGES)\r\na11 NAMESPACE\r\na12 LOGOUT\r\n")); err != nil {
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
		"a9 NO authentication required\r\n",
		"a10 NO authentication required\r\n",
		"a11 NO authentication required\r\n",
		"* BYE gogomail IMAP4rev1 server logging out\r\n",
		"a12 OK LOGOUT completed\r\n",
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
		"a2 OK LOGIN completed\r\n",
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 STATUS inbox (UIDNEXT RECENT)\r\na3 STATUS inbox (BADITEM)\r\na4 STATUS inbox MESSAGES\r\n")); err != nil {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a4 BAD STATUS requires parenthesized item list\r\n" {
		t.Fatalf("unparenthesized status line = %q err = %v", line, err)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if _, err := client.Write([]byte("a3 UID FETCH 7:8 (FLAGS) (CHANGEDSINCE 17)\r\na4 UID FETCH 7 (FLAGS) (CHANGEDSINCE nope)\r\n")); err != nil {
		t.Fatalf("write uid fetch changedsince: %v", err)
	}
	want := []string{
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41 MODSEQ (18))\r\n",
		"a3 OK UID FETCH completed\r\n",
		"a4 BAD FETCH CHANGEDSINCE modifier is invalid\r\n",
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
	if _, err := client.Write([]byte("a5 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read logout response: %v", err)
		}
		if line == "a5 OK LOGOUT completed\r\n" {
			break
		}
	}
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
	if _, err := client.Write([]byte("a3 UID FETCH 7:8 (FLAGS RFC822.SIZE)\r\na4 UID FETCH 1:* (FLAGS RFC822.SIZE)\r\na5 UID FETCH 999:* (FLAGS RFC822.SIZE)\r\n")); err != nil {
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
	if _, err := client.Write([]byte("a6 LOGOUT\r\n")); err != nil {
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
	if _, err := client.Write([]byte("a3 FETCH 1:* (FLAGS RFC822.SIZE)\r\na4 FETCH 999 (FLAGS)\r\na5 FETCH 1:999 (FLAGS)\r\n")); err != nil {
		t.Fatalf("write fetch: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41)\r\n",
		"a3 OK FETCH completed\r\n",
		"a4 BAD FETCH requires a valid message sequence set\r\n",
		"a5 BAD FETCH requires a valid message sequence set\r\n",
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
	if _, err := client.Write([]byte("a6 LOGOUT\r\n")); err != nil {
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
	if _, err := client.Write([]byte("a3 SEARCH ALL\r\na4 UID SEARCH ALL\r\na5 SEARCH UID 8:9\r\na6 SEARCH UNSEEN SINCE 04-May-2026 LARGER 20\r\na7 UID SEARCH ALL FROM archive SENTBEFORE 04-May-2026\r\na8 SEARCH NOT SEEN\r\na9 UID SEARCH OR FROM sender BCC hidden\r\na10 SEARCH CHARSET UTF-8 SUBJECT IMAP\r\na11 UID SEARCH CHARSET US-ASCII ALL\r\na12 SEARCH CHARSET ISO-8859-1 ALL\r\na13 SEARCH 2:*\r\na14 UID SEARCH 1:* SUBJECT Archive\r\na15 SEARCH (UNSEEN BCC hidden)\r\na16 UID SEARCH OR (SUBJECT IMAP) (BCC hidden)\r\na17 UID SEARCH MODSEQ 20\r\na18 SEARCH MODSEQ \"/flags/\\\\Seen\" all 17\r\na19 SEARCH MODSEQ \"/flags/\\\\Seen\" bogus 17\r\na20 SEARCH RETURN (MIN MAX COUNT) UNSEEN\r\na21 UID SEARCH RETURN (ALL COUNT) ALL\r\na22 SEARCH RETURN () ALL\r\na23 SEARCH RETURN (MIN) MODSEQ 20\r\na24 SEARCH RETURN (COUNT COUNT) ALL\r\na25 UID SEARCH RETURN (ALL COUNT) DELETED\r\na26 UID SEARCH UID 1:*\r\na27 UID SEARCH UID 999:*\r\na28 SEARCH (ALL)\r\na29 SEARCH ()\r\na30 SEARCH MODSEQ 20\"\r\na31 SEARCH MODSEQ \"/flags/\\\\Seen\" all\" 17\r\n")); err != nil {
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
		"a30 BAD SEARCH criteria are unsupported\r\n",
		"a31 BAD SEARCH criteria are unsupported\r\n",
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SORT (SUBJECT) UTF-8 ALL\r\na4 SORT (REVERSE DATE) UTF-8 ALL\r\na5 UID SORT (SIZE) US-ASCII ALL\r\na6 SORT (SUBJECT) UTF-8 SUBJECT Archive\r\na7 SORT (ARRIVAL) ISO-8859-1 ALL\r\na8 SORT (BOGUS) UTF-8 ALL\r\n")); err != nil {
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
	if _, err := client.Write([]byte("a9 LOGOUT\r\n")); err != nil {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 THREAD ORDEREDSUBJECT UTF-8 ALL\r\na4 UID THREAD ORDEREDSUBJECT US-ASCII SUBJECT Project\r\na5 THREAD ORDEREDSUBJECT ISO-8859-1 ALL\r\na6 THREAD REFERENCES UTF-8 ALL\r\n")); err != nil {
		t.Fatalf("write thread commands: %v", err)
	}
	want := []string{
		"* THREAD (1)(2 (3)(4))\r\n",
		"a3 OK THREAD completed\r\n",
		"* THREAD (12 (13)(14))\r\n",
		"a4 OK UID THREAD completed\r\n",
		"a5 NO [BADCHARSET (US-ASCII UTF-8)] THREAD charset is unsupported\r\n",
		"a6 BAD THREAD algorithm is unsupported\r\n",
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
	if _, err := client.Write([]byte("a7 LOGOUT\r\n")); err != nil {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
		t.Fatalf("login line = %q err = %v", line, err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 SEARCH RETURN (SAVE) UNSEEN\r\na4 FETCH $ (FLAGS)\r\na5 UID SEARCH UID $ SMALLER 50\r\na6 UID SEARCH RETURN (SAVE MIN) ALL\r\na7 UID FETCH $ (FLAGS)\r\na8 UID SEARCH RETURN (SAVE COUNT) DELETED\r\na9 FETCH $ (FLAGS)\r\na10 SELECT inbox\r\na11 FETCH $ (FLAGS)\r\n")); err != nil {
		t.Fatalf("write searchres commands: %v", err)
	}
	want := []string{
		"a3 OK SEARCH completed\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41)\r\n",
		"a4 OK FETCH completed\r\n",
		"* SEARCH 8\r\n",
		"a5 OK UID SEARCH completed\r\n",
		"* ESEARCH (TAG \"a6\") UID MIN 7\r\n",
		"a6 OK UID SEARCH completed\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"a7 OK UID FETCH completed\r\n",
		"* ESEARCH (TAG \"a8\") UID COUNT 0\r\n",
		"a8 OK UID SEARCH completed\r\n",
		"a9 OK FETCH completed\r\n",
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a11 OK FETCH completed\r\n" {
		t.Fatalf("fetch after select reset = %q err = %v", line, err)
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
	if _, err := client.Write([]byte("a3 SEARCH SINCE 05-May-2026\r\na4 UID SEARCH BEFORE 05-May-2026\r\na5 SEARCH ON 05-May-2026\r\na6 UID SEARCH SENTON 03-May-2026\r\na7 SEARCH SENTSINCE 04-May-2026\r\na8 UID SEARCH SENTBEFORE 04-May-2026\r\na9 SEARCH SINCE 05-May-2026\"\r\n")); err != nil {
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
		"a9 BAD SEARCH criteria are unsupported\r\n",
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
	if _, err := client.Write([]byte("a10 LOGOUT\r\n")); err != nil {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if _, err := client.Write([]byte("a4 UID FETCH 7 RFC822\r\n")); err != nil {
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
	if line, err = reader.ReadString('\n'); err != nil || line != "a4 OK UID FETCH completed\r\n" {
		t.Fatalf("rfc822 completion = %q err = %v", line, err)
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
	if _, err := client.Write([]byte("a5 LOGOUT\r\n")); err != nil {
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
	if _, err := client.Write([]byte("a4 UID FETCH 9 BODY.PEEK[HEADER.FIELDS.NOT (From)]<0.10>\r\n")); err != nil {
		t.Fatalf("write uid fetch partial header fields not: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read partial header fields not literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER]<0> {10}\r\n" {
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
	if _, err := client.Write([]byte("a5 LOGOUT\r\n")); err != nil {
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
	if _, err := client.Write([]byte("a4 UID FETCH 9 BODY.PEEK[HEADER.FIELDS (Subject From)]<0.14>\r\n")); err != nil {
		t.Fatalf("write uid fetch partial header fields: %v", err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read partial header fields literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER]<0> {14}\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if _, err := client.Write([]byte("a3 UID STORE 7 (UNCHANGEDSINCE 27) +FLAGS (\\Seen)\r\na4 UID STORE 7:8 (UNCHANGEDSINCE 27) +FLAGS (\\Seen)\r\na5 UID STORE 7 (UNCHANGEDSINCE nope) +FLAGS (\\Seen)\r\n")); err != nil {
		t.Fatalf("write uid store unchanged since: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen) MODSEQ (27))\r\n",
		"a3 OK UID STORE completed\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen) MODSEQ (27))\r\n",
		"a4 OK [MODIFIED 8] UID STORE conditional store completed\r\n",
		"a5 BAD UID STORE UNCHANGEDSINCE modifier is invalid\r\n",
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
	if _, err := client.Write([]byte("a6 LOGOUT\r\n")); err != nil {
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
	if _, err := client.Write([]byte("a3 STORE 1 (UNCHANGEDSINCE 27) +FLAGS.SILENT (\\Seen)\r\na4 STORE 1:2 (UNCHANGEDSINCE 27) +FLAGS.SILENT (\\Seen)\r\n")); err != nil {
		t.Fatalf("write store unchanged since: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen) MODSEQ (27))\r\n",
		"a3 OK STORE completed\r\n",
		"* 1 FETCH (UID 7 FLAGS (\\Seen) MODSEQ (27))\r\n",
		"a4 OK [MODIFIED 2] STORE conditional store completed\r\n",
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
	if _, err := client.Write([]byte("a5 LOGOUT\r\n")); err != nil {
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
	if _, err := parseIMAPFields("a1 LOGIN user@example.com {6}"); err == nil {
		t.Fatal("parseIMAPFields accepted unsupported literal")
	}
	if _, err := parseIMAPFields("a1 LOGIN user@example.com {6+}"); err == nil {
		t.Fatal("parseIMAPFields accepted non-synchronizing literal")
	}
	literal := "secret value"
	fields, err := parseIMAPFieldsWithLiteral("a1 LOGIN user@example.com {12}", &literal)
	if err != nil {
		t.Fatalf("parseIMAPFieldsWithLiteral returned error: %v", err)
	}
	if got, want := fields, []string{"a1", "LOGIN", "user@example.com", literal}; !reflect.DeepEqual(got, want) {
		t.Fatalf("literal fields = %#v, want %#v", got, want)
	}
	fields, err = parseIMAPFieldsWithLiteral("a1 APPEND inbox {12+}", &literal)
	if err != nil {
		t.Fatalf("parseIMAPFieldsWithLiteral literal+ returned error: %v", err)
	}
	if got, want := fields, []string{"a1", "APPEND", "inbox", literal}; !reflect.DeepEqual(got, want) {
		t.Fatalf("literal+ fields = %#v, want %#v", got, want)
	}
	fields, err = parseIMAPFields(`a1 SEARCH SUBJECT "Project \"Q2\""`)
	if err != nil {
		t.Fatalf("parseIMAPFields quoted string with escaped quote returned error: %v", err)
	}
	if got, want := fields, []string{"a1", "SEARCH", "SUBJECT", `Project "Q2"`}; !reflect.DeepEqual(got, want) {
		t.Fatalf("escaped quoted fields = %#v, want %#v", got, want)
	}
	if _, _, ok, err := imapCommandLiteralSize("a1 APPEND inbox {12}\r\n"); err != nil || !ok {
		t.Fatalf("imapCommandLiteralSize synchronizing ok = %v err = %v", ok, err)
	}
	if size, nonSync, ok, err := imapCommandLiteralSize("a1 APPEND inbox {12+}\r\n"); err != nil || !ok || !nonSync || size != 12 {
		t.Fatalf("imapCommandLiteralSize literal+ = size %d nonSync %v ok %v err %v", size, nonSync, ok, err)
	}
}

func TestIMAPQuotedStringPreservesIdentitySpacing(t *testing.T) {
	t.Parallel()

	got := imapQuotedString("Project  \"Q2\"\\Draft\t")
	want := `"Project  \"Q2\"\\Draft "`
	if got != want {
		t.Fatalf("imapQuotedString = %q, want %q", got, want)
	}
}

func TestIMAPAppendOptionsParseFlagsAndInternalDate(t *testing.T) {
	t.Parallel()

	flags, internalDate, ok := imapAppendOptions([]string{`(\Seen`, `\Deleted)`, "5-May-2026 12:34:56 +0900"})
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
	for _, value := range []string{"4", "1:4"} {
		if got, ok := parseIMAPSequenceSet(value, 3); ok {
			t.Fatalf("parseIMAPSequenceSet(%q, 3) = %v true, want out-of-range rejection", value, got)
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

func TestIMAPMailboxDisplayNameTrimsStoredRootPrefix(t *testing.T) {
	t.Parallel()

	got := imapMailboxDisplayName(Mailbox{ID: "mailbox-1", FullPath: "/Archive/2026"})
	if got != "Archive/2026" {
		t.Fatalf("display name = %q, want Archive/2026", got)
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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
	if line, err := reader.ReadString('\n'); err != nil || line != "a1 OK LOGIN completed\r\n" {
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

func (b *operationalMailboxNameBackend) CopyMessages(_ context.Context, req CopyMessagesRequest) ([]MessageSummary, error) {
	b.copyDest = req.DestMailboxID
	return []MessageSummary{{ID: "copy-50", MailboxID: req.DestMailboxID, UID: 50}}, nil
}

func (b *operationalMailboxNameBackend) MoveMessages(_ context.Context, req MoveMessagesRequest) ([]MoveMessageResult, error) {
	b.moveDest = req.DestMailboxID
	return []MoveMessageResult{{
		Source:              MessageSummary{ID: "message-7", MailboxID: req.SourceMailboxID, UID: 7, SequenceNumber: 1},
		Destination:         MessageSummary{ID: "move-51", MailboxID: req.DestMailboxID, UID: 51, SequenceNumber: 1},
		SourceHighestModSeq: 30,
	}}, nil
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

type threadBackend struct {
	fakeBackend
}

func (threadBackend) GetMailbox(_ context.Context, _ UserID, _ MailboxID) (Mailbox, error) {
	return Mailbox{ID: "inbox", Name: "INBOX", UIDValidity: 1, UIDNext: 15, Messages: 4, Unseen: 4}, nil
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
		if req.UnchangedSince > 0 && modseq > req.UnchangedSince {
			modified = append(modified, uid)
			continue
		}
		summaries = append(summaries, MessageSummary{ID: MessageID(fmt.Sprintf("message-%d", uid)), UID: uid, SequenceNumber: uint32(uid - 6), Flags: MessageFlags{Read: req.Flags.Read, Starred: req.Flags.Starred, Answered: req.Flags.Answered, Draft: req.Flags.Draft, Deleted: req.Flags.Deleted}, ModSeq: modseq})
	}
	if len(modified) > 0 {
		return summaries, &StoreModifiedError{UIDs: modified, Summaries: summaries}
	}
	return summaries, nil
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

func (fakeBackend) CopyMessages(context.Context, CopyMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{{ID: "message-copy-1", MailboxID: "inbox", UID: 9}}, nil
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

func (b *copyBackend) CopyMessages(_ context.Context, req CopyMessagesRequest) ([]MessageSummary, error) {
	b.requests = append(b.requests, req)
	if b.nextUID == 0 {
		b.nextUID = 9
	}
	summaries := make([]MessageSummary, 0, len(req.UIDs))
	for range req.UIDs {
		summaries = append(summaries, MessageSummary{ID: MessageID(fmt.Sprintf("message-copy-%d", b.nextUID)), MailboxID: req.DestMailboxID, UID: b.nextUID})
		b.nextUID++
	}
	return summaries, nil
}

type selectedCopyBackend struct {
	fakeBackend
}

func (selectedCopyBackend) CopyMessages(_ context.Context, req CopyMessagesRequest) ([]MessageSummary, error) {
	return []MessageSummary{{
		ID:             "message-copy-11",
		MailboxID:      req.DestMailboxID,
		UID:            11,
		SequenceNumber: 5,
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

type failingSubscribeBackend struct {
	fakeBackend
}

func (failingSubscribeBackend) Subscribe(context.Context, UserID, MailboxID) (<-chan MailboxEvent, func(), error) {
	return nil, nil, errors.New("subscription unavailable")
}

type missingMailboxBackend struct {
	fakeBackend
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
