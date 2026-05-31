package smtpd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	gosmtp "github.com/emersion/go-smtp"
)

func TestServerOptionDefaults(t *testing.T) {
	t.Parallel()

	if got := durationOrDefault(0, 30*time.Second); got != 30*time.Second {
		t.Fatalf("duration default = %s", got)
	}
	if got := int64OrDefault(0, 25); got != 25 {
		t.Fatalf("int64 default = %d", got)
	}
	if got := intOrDefault(0, 100); got != 100 {
		t.Fatalf("int default = %d", got)
	}
}

func TestServerOptionOverrides(t *testing.T) {
	t.Parallel()

	if got := durationOrDefault(10*time.Second, 30*time.Second); got != 10*time.Second {
		t.Fatalf("duration override = %s", got)
	}
	if got := int64OrDefault(42, 25); got != 42 {
		t.Fatalf("int64 override = %d", got)
	}
	if got := intOrDefault(7, 100); got != 7 {
		t.Fatalf("int override = %d", got)
	}
}

func TestServerImplicitTLSOption(t *testing.T) {
	t.Parallel()

	server := newSMTPServer(gosmtp.BackendFunc(func(*gosmtp.Conn) (gosmtp.Session, error) {
		return nil, nil
	}), ServerOptions{
		Addr:        "127.0.0.1:0",
		Domain:      "mail.example",
		ImplicitTLS: true,
	})
	if server.Addr != "127.0.0.1:0" {
		t.Fatalf("Addr = %q", server.Addr)
	}
}

func TestRunServerRejectsImplicitTLSWithoutConfig(t *testing.T) {
	t.Parallel()

	err := RunServer(context.Background(), ServerOptions{
		Addr:        "127.0.0.1:0",
		Domain:      "mail.example",
		ImplicitTLS: true,
		Backend: gosmtp.BackendFunc(func(*gosmtp.Conn) (gosmtp.Session, error) {
			return nil, nil
		}),
	})
	if err == nil || !strings.Contains(err.Error(), "requires TLS configuration") {
		t.Fatalf("RunServer error = %v, want TLS configuration rejection", err)
	}
}

func TestRunServerRejectsNegativeMaxConnections(t *testing.T) {
	t.Parallel()

	err := RunServer(context.Background(), ServerOptions{
		Addr:           "127.0.0.1:0",
		Domain:         "mail.example",
		MaxConnections: -1,
		Backend: gosmtp.BackendFunc(func(*gosmtp.Conn) (gosmtp.Session, error) {
			return nil, nil
		}),
	})
	if err == nil || !strings.Contains(err.Error(), "max connections") {
		t.Fatalf("RunServer error = %v, want max connections rejection", err)
	}
}

func TestSMTPConnectionLimitListenerRejectsExcessConnections(t *testing.T) {
	t.Parallel()

	base, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() { _ = base.Close() })
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	limited := newSMTPConnectionLimitListener(base, 1, logger)

	firstAccept := make(chan net.Conn, 1)
	go func() {
		conn, _ := limited.Accept()
		firstAccept <- conn
	}()
	client1, err := net.Dial("tcp", base.Addr().String())
	if err != nil {
		t.Fatalf("first Dial returned error: %v", err)
	}
	t.Cleanup(func() { _ = client1.Close() })
	server1 := <-firstAccept
	t.Cleanup(func() { _ = server1.Close() })

	secondAccept := make(chan net.Conn, 1)
	go func() {
		conn, _ := limited.Accept()
		secondAccept <- conn
	}()
	client2, err := net.Dial("tcp", base.Addr().String())
	if err != nil {
		t.Fatalf("second Dial returned error: %v", err)
	}
	defer client2.Close()
	_ = client2.SetReadDeadline(time.Now().Add(time.Second))
	line, err := bufio.NewReader(client2).ReadString('\n')
	if err != nil {
		t.Fatalf("read over-limit banner returned error: %v", err)
	}
	if !strings.HasPrefix(line, "421 4.3.2 Too many connections") {
		t.Fatalf("over-limit banner = %q", line)
	}
	gotLog := logs.String()
	if !strings.Contains(gotLog, "smtp connection rejected") || !strings.Contains(gotLog, "connection_limit") {
		t.Fatalf("connection-limit log = %q, want rejection context", gotLog)
	}

	if err := server1.Close(); err != nil {
		t.Fatalf("first server close returned error: %v", err)
	}
	client3, err := net.Dial("tcp", base.Addr().String())
	if err != nil {
		t.Fatalf("third Dial returned error: %v", err)
	}
	defer client3.Close()
	select {
	case server3 := <-secondAccept:
		if server3 == nil {
			t.Fatal("third accepted connection = nil")
		}
		_ = server3.Close()
	case <-time.After(time.Second):
		t.Fatal("third connection was not accepted after slot release")
	}
}

func TestRunServerRejectsImplicitTLSWithoutCertificate(t *testing.T) {
	t.Parallel()

	err := RunServer(context.Background(), ServerOptions{
		Addr:        "127.0.0.1:0",
		Domain:      "mail.example",
		ImplicitTLS: true,
		TLSConfig:   &tls.Config{MinVersion: tls.VersionTLS12},
		Backend: gosmtp.BackendFunc(func(*gosmtp.Conn) (gosmtp.Session, error) {
			return nil, nil
		}),
	})
	if err == nil || !strings.Contains(err.Error(), "server certificate") {
		t.Fatalf("RunServer error = %v, want server certificate rejection", err)
	}
}

func TestHasServerCertificateAllowsDynamicCallbacks(t *testing.T) {
	t.Parallel()

	if !hasServerCertificate(&tls.Config{GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return nil, nil
	}}) {
		t.Fatal("GetCertificate TLS config was rejected")
	}
	if !hasServerCertificate(&tls.Config{GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return nil, nil
	}}) {
		t.Fatal("GetConfigForClient TLS config was rejected")
	}
	if hasServerCertificate(&tls.Config{}) {
		t.Fatal("empty TLS config accepted as having a server certificate")
	}
}

func TestServerTLSConfigIsClonedAndMinVersionHardened(t *testing.T) {
	t.Parallel()

	original := &tls.Config{MinVersion: tls.VersionTLS10}
	server := newSMTPServer(gosmtp.BackendFunc(func(*gosmtp.Conn) (gosmtp.Session, error) {
		return nil, nil
	}), ServerOptions{
		Addr:      "127.0.0.1:0",
		Domain:    "mail.example",
		TLSConfig: original,
	})

	if server.TLSConfig == nil {
		t.Fatal("TLSConfig = nil")
	}
	if server.TLSConfig == original {
		t.Fatal("TLSConfig was not cloned")
	}
	if server.TLSConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %x, want TLS 1.2", server.TLSConfig.MinVersion)
	}
	if original.MinVersion != tls.VersionTLS10 {
		t.Fatalf("original MinVersion mutated to %x", original.MinVersion)
	}
}
