package dsn

import (
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

func TestComposeDeliveryStatusNotification(t *testing.T) {
	composed, err := Compose(Report{
		ReportingMTA: "mx.example.com",
		OriginalID:   "env-123",
		From:         outbound.Address{Name: "Mail Delivery Subsystem", Email: "mailer-daemon@example.com"},
		To:           outbound.Address{Email: "sender@example.net"},
		MessageID:    "<dsn@example.com>",
		Date:         time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC),
		Recipients: []RecipientStatus{{
			Recipient:         "User@Example.NET",
			OriginalRecipient: "rfc822; Alias@Example.NET\r\n",
			Action:            "failed",
			Status:            "5.1.1",
			RemoteMTA:         "mx.example.net\r\n",
			Diagnostic:        "550 5.1.1 user unknown\r\nsecond line",
			LastAttemptAt:     time.Date(2026, 5, 3, 12, 1, 0, 0, time.UTC),
		}},
	})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	raw := string(composed.Raw)
	for _, want := range []string{
		"Auto-Submitted: auto-replied",
		"Content-Type: multipart/report; report-type=delivery-status;",
		"Content-Type: message/delivery-status",
		"Reporting-MTA: dns; mx.example.com",
		"Original-Envelope-Id: env-123",
		"Original-Recipient: rfc822; Alias@Example.NET",
		"Final-Recipient: rfc822; user@example.net",
		"Action: failed",
		"Status: 5.1.1",
		"Remote-MTA: dns; mx.example.net",
		"Diagnostic-Code: smtp; 550 5.1.1 user unknown second line",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("DSN raw missing %q:\n%s", want, raw)
		}
	}
}

func TestComposeRequiresRecipientStatus(t *testing.T) {
	_, err := Compose(Report{ReportingMTA: "mx.example.com"})
	if err == nil {
		t.Fatal("Compose() error = nil, want recipient requirement")
	}
}

func TestComposeRejectsInvalidRecipientStatus(t *testing.T) {
	t.Parallel()

	_, err := Compose(Report{
		ReportingMTA: "mx.example.com",
		Recipients: []RecipientStatus{{
			Recipient: "user@example.net",
			Action:    "lost",
			Status:    "5.1.1",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid dsn action") {
		t.Fatalf("Compose() error = %v, want invalid action", err)
	}

	_, err = Compose(Report{
		ReportingMTA: "mx.example.com",
		Recipients: []RecipientStatus{{
			Recipient: "user@example.net",
			Action:    "failed",
			Status:    "500",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid dsn status") {
		t.Fatalf("Compose() error = %v, want invalid status", err)
	}
}
