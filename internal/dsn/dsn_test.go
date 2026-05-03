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
			Recipient:     "User@Example.NET",
			Action:        "failed",
			Status:        "5.1.1",
			RemoteMTA:     "mx.example.net",
			Diagnostic:    "550 5.1.1 user unknown\r\nsecond line",
			LastAttemptAt: time.Date(2026, 5, 3, 12, 1, 0, 0, time.UTC),
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
		"Final-Recipient: rfc822; user@example.net",
		"Action: failed",
		"Status: 5.1.1",
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
