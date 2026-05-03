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

	_, err = Compose(Report{
		ReportingMTA: "mx.example.com",
		Recipients: []RecipientStatus{{
			Recipient: "user@example.net",
			Action:    "delivered",
			Status:    "5.0.0",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match action") {
		t.Fatalf("Compose() error = %v, want mismatched status/action", err)
	}

	for _, invalid := range []string{"05.1.1", "5.1000.1", "5.1.1000", "3.1.1"} {
		_, err = Compose(Report{
			ReportingMTA: "mx.example.com",
			Recipients: []RecipientStatus{{
				Recipient: "user@example.net",
				Action:    "failed",
				Status:    invalid,
			}},
		})
		if err == nil || !strings.Contains(err.Error(), "invalid dsn status") {
			t.Fatalf("Compose() status %q error = %v, want invalid status", invalid, err)
		}
	}
}

func TestComposeDefaultsStatusByAction(t *testing.T) {
	t.Parallel()

	composed, err := Compose(Report{
		ReportingMTA: "mx.example.com",
		Recipients: []RecipientStatus{
			{Recipient: "delivered@example.net", Action: "delivered"},
			{Recipient: "delayed@example.net", Action: "delayed"},
			{Recipient: "failed@example.net", Action: "failed"},
		},
	})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	raw := string(composed.Raw)
	for _, want := range []string{
		"Action: delivered\r\nStatus: 2.0.0",
		"Action: delayed\r\nStatus: 4.0.0",
		"Action: failed\r\nStatus: 5.0.0",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("DSN raw missing %q:\n%s", want, raw)
		}
	}
}

func TestComposeSanitizesBoundaryAndFinalRecipient(t *testing.T) {
	t.Parallel()

	composed, err := Compose(Report{
		ReportingMTA: "mx.example.com",
		From:         outbound.Address{Email: "mailer-daemon@example.com"},
		To:           outbound.Address{Email: "sender@example.net"},
		MessageID:    "<dsn/\r\nbad@example.com>",
		Date:         time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC),
		Recipients: []RecipientStatus{{
			Recipient: "User@Example.NET\r\nInjected: bad",
			Action:    "failed",
			Status:    "5.1.1",
		}},
	})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	raw := string(composed.Raw)
	if strings.Contains(raw, "gogomail-dsn-dsn/\r\nbad@example.com") {
		t.Fatalf("boundary was not sanitized:\n%s", raw)
	}
	if !strings.Contains(raw, `boundary="gogomail-dsn-dsn-bad@example.com"`) {
		t.Fatalf("raw missing sanitized boundary:\n%s", raw)
	}
	if !strings.Contains(raw, "Final-Recipient: rfc822; user@example.net injected: bad") {
		t.Fatalf("raw missing sanitized final recipient:\n%s", raw)
	}
	if strings.Contains(raw, "\r\nInjected: bad") {
		t.Fatalf("raw contains injected header:\n%s", raw)
	}
}

func TestComposeCapsBoundaryLength(t *testing.T) {
	t.Parallel()

	composed, err := Compose(Report{
		ReportingMTA: "mx.example.com",
		MessageID:    "<" + strings.Repeat("a", 120) + "@example.com>",
		Recipients: []RecipientStatus{{
			Recipient: "user@example.net",
			Action:    "failed",
			Status:    "5.1.1",
		}},
	})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	raw := string(composed.Raw)
	prefix := `boundary="gogomail-dsn-`
	start := strings.Index(raw, prefix)
	if start < 0 {
		t.Fatalf("raw missing boundary:\n%s", raw)
	}
	start += len(`boundary="`)
	end := strings.Index(raw[start:], `"`)
	if end < 0 {
		t.Fatalf("raw missing closing boundary quote:\n%s", raw)
	}
	boundary := raw[start : start+end]
	if len(boundary) > 70 {
		t.Fatalf("boundary length = %d, want <= 70: %q", len(boundary), boundary)
	}
}

func TestComposeSanitizesReturnedMessageID(t *testing.T) {
	t.Parallel()

	composed, err := Compose(Report{
		ReportingMTA: "mx.example.com",
		MessageID:    "<dsn@example.com>\r\nInjected: bad",
		Recipients: []RecipientStatus{{
			Recipient: "user@example.net",
			Action:    "failed",
			Status:    "5.1.1",
		}},
	})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	if strings.ContainsAny(composed.MessageID, "\r\n ") {
		t.Fatalf("MessageID = %q, want sanitized metadata value", composed.MessageID)
	}
	if strings.Contains(string(composed.Raw), "\r\nInjected: bad") {
		t.Fatalf("raw contains injected Message-ID header:\n%s", string(composed.Raw))
	}
}

func TestComposeSanitizesAddressHeaders(t *testing.T) {
	t.Parallel()

	composed, err := Compose(Report{
		ReportingMTA: "mx.example.com",
		From:         outbound.Address{Name: "Mailer\r\nBad: yes", Email: "mailer-daemon@example.com\r\nBad: yes"},
		To:           outbound.Address{Email: "sender@example.net\r\nBad: yes"},
		Recipients: []RecipientStatus{{
			Recipient: "user@example.net",
			Action:    "failed",
			Status:    "5.1.1",
		}},
	})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	raw := string(composed.Raw)
	if strings.Contains(raw, "\r\nBad: yes") {
		t.Fatalf("raw contains injected address header:\n%s", raw)
	}
	if !strings.Contains(raw, "<sender@example.netBad:yes>") {
		t.Fatalf("raw missing sanitized To address:\n%s", raw)
	}
}
