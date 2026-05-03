package delivery

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDeliveryAttemptEventPayload(t *testing.T) {
	t.Parallel()

	raw, err := deliveryAttemptEventPayload(Attempt{
		MessageID:       "018f0000-0000-7000-8000-000000000001",
		RFCMessageID:    "<msg@example.com>",
		CompanyID:       "company-1",
		DomainID:        "domain-1",
		Farm:            "general",
		Recipient:       "user@example.net",
		RecipientDomain: "example.net",
		Status:          AttemptBounced,
		ErrorMessage:    "550 no such user",
		AttemptedAt:     time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
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
}
