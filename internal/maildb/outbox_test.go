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
}
