package smtpd

import (
	"context"
	"errors"
	"testing"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/gogomail/gogomail/internal/storage"
)

func TestStaticTrustedRelaysAllowsCIDRAndPlainIP(t *testing.T) {
	t.Parallel()

	relays, err := NewStaticTrustedRelays([]string{"192.0.2.0/24", "2001:db8::1"})
	if err != nil {
		t.Fatalf("NewStaticTrustedRelays returned error: %v", err)
	}
	for _, remote := range []string{"192.0.2.25", "2001:db8::1"} {
		allowed, err := relays.AllowRelay(context.Background(), remote)
		if err != nil {
			t.Fatalf("AllowRelay returned error: %v", err)
		}
		if !allowed {
			t.Fatalf("AllowRelay(%q) = false, want true", remote)
		}
	}
}

func TestStaticTrustedRelaysAllowsRemoteAddrWithPort(t *testing.T) {
	t.Parallel()

	relays, err := NewStaticTrustedRelays([]string{"192.0.2.0/24", "2001:db8::/32"})
	if err != nil {
		t.Fatalf("NewStaticTrustedRelays returned error: %v", err)
	}
	for _, remote := range []string{"192.0.2.25:2525", "[2001:db8::1]:2525"} {
		allowed, err := relays.AllowRelay(context.Background(), remote)
		if err != nil {
			t.Fatalf("AllowRelay returned error: %v", err)
		}
		if !allowed {
			t.Fatalf("AllowRelay(%q) = false, want true", remote)
		}
	}
}

func TestStaticTrustedRelaysAllowsIPv4MappedRemoteAddr(t *testing.T) {
	t.Parallel()

	relays, err := NewStaticTrustedRelays([]string{"192.0.2.0/24"})
	if err != nil {
		t.Fatalf("NewStaticTrustedRelays returned error: %v", err)
	}
	for _, remote := range []string{"::ffff:192.0.2.25", "[::ffff:192.0.2.25]:2525"} {
		allowed, err := relays.AllowRelay(context.Background(), remote)
		if err != nil {
			t.Fatalf("AllowRelay returned error: %v", err)
		}
		if !allowed {
			t.Fatalf("AllowRelay(%q) = false, want true", remote)
		}
	}
}

func TestStaticTrustedRelaysRejectsUntrustedRemote(t *testing.T) {
	t.Parallel()

	relays, err := NewStaticTrustedRelays([]string{"192.0.2.0/24"})
	if err != nil {
		t.Fatalf("NewStaticTrustedRelays returned error: %v", err)
	}
	allowed, err := relays.AllowRelay(context.Background(), "198.51.100.1")
	if err != nil {
		t.Fatalf("AllowRelay returned error: %v", err)
	}
	if allowed {
		t.Fatal("AllowRelay accepted untrusted remote")
	}
}

func TestStaticTrustedRelaysRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	if _, err := NewStaticTrustedRelays([]string{"not a cidr"}); err == nil {
		t.Fatal("NewStaticTrustedRelays accepted invalid trusted relay")
	}
}

func TestSessionRejectsUntrustedRelay(t *testing.T) {
	t.Parallel()

	relays, err := NewStaticTrustedRelays([]string{"192.0.2.0/24"})
	if err != nil {
		t.Fatalf("NewStaticTrustedRelays returned error: %v", err)
	}
	receiver := NewReceiver(ReceiverOptions{
		Store:           storage.NewLocalStore(t.TempDir()),
		Resolver:        StaticResolver{"user@example.com": {CompanyID: "c", DomainID: "d", UserID: "u", Address: "user@example.com"}},
		RelayAuthorizer: relays,
	})
	rawSession, err := receiver.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	session := rawSession.(*session)
	session.remoteAddr = "198.51.100.1"

	err = session.Mail("sender@example.net", nil)
	if err == nil {
		t.Fatal("Mail accepted untrusted relay")
	}
	var smtpErr *gosmtp.SMTPError
	if !errors.As(err, &smtpErr) || smtpErr.Code != 550 {
		t.Fatalf("Mail error = %v, want SMTP 550", err)
	}
}
