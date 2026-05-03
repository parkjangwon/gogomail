package maildb

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/message"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestStoredEventPayload(t *testing.T) {
	t.Parallel()

	payload, err := storedEventPayload("msg-1", smtpd.ReceivedMessage{
		EnvelopeFrom: "sender@example.net",
		Mailbox: smtpd.Mailbox{
			CompanyID: "company-1",
			DomainID:  "domain-1",
			UserID:    "user-1",
			Address:   "admin@example.com",
		},
		StoragePath: "mailstore/example.eml",
		Parsed: message.ParsedMessage{
			MessageID: "<abc@example.com>",
			Subject:   "hello",
		},
		ReceivedAt: time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		Size:       123,
		DSN: smtpd.DSNOptions{
			Return:     "HDRS",
			EnvelopeID: "env-1",
			Recipients: []smtpd.DSNRecipientOptions{{
				Address:           "admin@example.com",
				Notify:            []string{"FAILURE"},
				OriginalRecipient: "rfc822; alias@example.com",
			}},
		},
		Authentication: smtpd.AuthenticationResults{
			AuthservID: "mx.example.com",
			SPF:        smtpd.AuthCheckResult{Result: smtpd.AuthResultPass, Identifier: "sender@example.net"},
		},
	})
	if err != nil {
		t.Fatalf("storedEventPayload returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if got["event"] != "mail.stored" {
		t.Fatalf("event = %v", got["event"])
	}
	if got["message_id"] != "msg-1" {
		t.Fatalf("message_id = %v", got["message_id"])
	}
	if got["storage_path"] != "mailstore/example.eml" {
		t.Fatalf("storage_path = %v", got["storage_path"])
	}
	if got["envelope_from"] != "sender@example.net" {
		t.Fatalf("envelope_from = %v", got["envelope_from"])
	}
	dsn, ok := got["dsn"].(map[string]any)
	if !ok || dsn["envelope_id"] != "env-1" {
		t.Fatalf("dsn = %+v", got["dsn"])
	}
	auth, ok := got["authentication_results"].(map[string]any)
	if !ok || auth["authserv_id"] != "mx.example.com" {
		t.Fatalf("authentication_results = %+v", got["authentication_results"])
	}
}
