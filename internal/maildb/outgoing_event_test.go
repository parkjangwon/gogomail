package maildb

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestOutgoingEventPayloadCarriesDSNOptions(t *testing.T) {
	t.Parallel()

	raw, err := outgoingEventPayload("msg-1", OutgoingMessage{
		CompanyID:    "company-1",
		DomainID:     "domain-1",
		UserID:       "user-1",
		RFCMessageID: "<queued@example.com>",
		Subject:      "hello",
		From:         outbound.Address{Email: "sender@example.com"},
		To:           []outbound.Address{{Email: "recipient@example.net"}},
		Farm:         outbound.FarmGeneral,
		StoragePath:  "mailstore/msg.eml",
		SentAt:       time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		DSN: smtpd.DSNOptions{
			Return:     "FULL",
			EnvelopeID: "env-queued",
			Recipients: []smtpd.DSNRecipientOptions{{
				Address:           "recipient@example.net",
				Notify:            []string{"FAILURE", "DELAY"},
				OriginalRecipient: "rfc822; alias@example.net",
			}},
		},
	})
	if err != nil {
		t.Fatalf("outgoingEventPayload returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	dsn, ok := got["dsn"].(map[string]any)
	if !ok {
		t.Fatalf("dsn = %+v", got["dsn"])
	}
	if dsn["return"] != "FULL" || dsn["envelope_id"] != "env-queued" {
		t.Fatalf("dsn = %+v", dsn)
	}
	recipients, ok := dsn["recipients"].([]any)
	if !ok || len(recipients) != 1 {
		t.Fatalf("dsn recipients = %+v", dsn["recipients"])
	}
}
