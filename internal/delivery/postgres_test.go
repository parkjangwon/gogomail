package delivery

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDeliveryAttemptEventPayload(t *testing.T) {
	t.Parallel()

	raw, err := deliveryAttemptEventPayload(Attempt{
		MessageID:         "018f0000-0000-7000-8000-000000000001",
		RFCMessageID:      "<msg@example.com>",
		CompanyID:         "company-1",
		DomainID:          "domain-1",
		Farm:              "general",
		Sender:            "sender@example.com",
		Recipient:         "user@example.net",
		RecipientDomain:   "example.net",
		Status:            AttemptBounced,
		EnhancedStatus:    "5.1.1",
		ErrorMessage:      "550 no such user",
		AttemptedAt:       time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		DSNReturn:         "HDRS",
		DSNEnvelopeID:     "env+2D1",
		DSNNotify:         []string{"FAILURE", "DELAY"},
		OriginalRecipient: "rfc822;alias+40example.net",
	})
	if err != nil {
		t.Fatalf("deliveryAttemptEventPayload returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if got["event"] != "mail.bounced" {
		t.Fatalf("event = %v, want mail.bounced", got["event"])
	}
	if got["recipient"] != "user@example.net" {
		t.Fatalf("recipient = %v", got["recipient"])
	}
	if got["sender"] != "sender@example.com" {
		t.Fatalf("sender = %v, want original sender", got["sender"])
	}
	if got["enhanced_status"] != "5.1.1" {
		t.Fatalf("enhanced_status = %v, want 5.1.1", got["enhanced_status"])
	}
	dsn, ok := got["dsn"].(map[string]any)
	if !ok {
		t.Fatalf("dsn = %#v, want object", got["dsn"])
	}
	if dsn["return"] != "HDRS" || dsn["envelope_id"] != "env+2D1" || dsn["original_recipient"] != "rfc822;alias+40example.net" {
		t.Fatalf("dsn = %#v, want DSN envelope metadata", dsn)
	}
	notify, ok := dsn["notify"].([]any)
	if !ok || len(notify) != 2 || notify[0] != "FAILURE" || notify[1] != "DELAY" {
		t.Fatalf("dsn notify = %#v", dsn["notify"])
	}
}
