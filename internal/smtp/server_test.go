package smtpd

import (
	"context"
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
