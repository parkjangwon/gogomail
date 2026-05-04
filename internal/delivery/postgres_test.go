package delivery

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

func TestDeliveryAttemptEventPayload(t *testing.T) {
	t.Parallel()

	raw, err := deliveryAttemptEventPayload(Attempt{
		MessageID:         " 018f0000-0000-7000-8000-000000000001 ",
		RFCMessageID:      " <msg@example.com> ",
		CompanyID:         " company-1 ",
		DomainID:          " domain-1 ",
		Farm:              " general ",
		Sender:            " sender@example.com ",
		Recipient:         " user@example.net ",
		RecipientDomain:   " example.net ",
		Status:            AttemptBounced,
		EnhancedStatus:    " 5.1.1 ",
		ErrorMessage:      " 550 no such user ",
		AttemptedAt:       time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
		DSNReturn:         " HDRS ",
		DSNEnvelopeID:     " env+2D1 ",
		DSNNotify:         []string{" FAILURE ", "", " DELAY "},
		OriginalRecipient: " rfc822;alias+40example.net ",
		StoragePath:       " mailstore/msg.eml ",
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
	if got["message_id"] != "018f0000-0000-7000-8000-000000000001" || got["company_id"] != "company-1" || got["domain_id"] != "domain-1" {
		t.Fatalf("ids = %#v", got)
	}
	if got["recipient"] != "user@example.net" {
		t.Fatalf("recipient = %v", got["recipient"])
	}
	if got["sender"] != "sender@example.com" {
		t.Fatalf("sender = %v, want original sender", got["sender"])
	}
	if got["storage_path"] != "mailstore/msg.eml" {
		t.Fatalf("storage_path = %v, want original message path", got["storage_path"])
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

func TestExhaustedEventPayloadNormalizesMetadata(t *testing.T) {
	t.Parallel()

	raw, err := exhaustedEventPayload(QueuedMessage{
		MessageID:    " msg-1 ",
		RFCMessageID: " <msg@example.com> ",
		CompanyID:    " company-1 ",
		DomainID:     " domain-1 ",
		Farm:         " general ",
		From:         outbound.Address{Email: " sender@example.com "},
		To:           []outbound.Address{{Email: " user@example.net "}},
	}, " temporary failure ")
	if err != nil {
		t.Fatalf("exhaustedEventPayload returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if got["message_id"] != "msg-1" || got["sender"] != "sender@example.com" || got["error_message"] != "temporary failure" {
		t.Fatalf("payload = %#v", got)
	}
	recipients, ok := got["recipients"].([]any)
	if !ok || len(recipients) != 1 || recipients[0] != "user@example.net" {
		t.Fatalf("recipients = %#v", got["recipients"])
	}
}

func TestShouldSuppressBouncedRecipientSkipsNullReversePath(t *testing.T) {
	t.Parallel()

	if shouldSuppressBouncedRecipient(Attempt{Status: AttemptBounced}) {
		t.Fatal("null reverse-path DSN bounce should not create suppression entries")
	}
	if !shouldSuppressBouncedRecipient(Attempt{Status: AttemptBounced, Sender: "sender@example.com"}) {
		t.Fatal("regular hard bounce should create suppression entries")
	}
	if shouldSuppressBouncedRecipient(Attempt{Status: AttemptFailed, Sender: "sender@example.com"}) {
		t.Fatal("temporary failure should not create suppression entries")
	}
}

func TestDeliveryAttemptDiagnosticsBoundsFields(t *testing.T) {
	t.Parallel()

	diagnostics, err := deliveryAttemptDiagnostics(Attempt{
		Sender:            strings.Repeat("s", 400),
		EnhancedStatus:    strings.Repeat("e", 80),
		DSNReturn:         " HDRS ",
		DSNEnvelopeID:     strings.Repeat("i", 120),
		DSNNotify:         []string{"FAILURE", "DELAY"},
		OriginalRecipient: strings.Repeat("o", 600),
	})
	if err != nil {
		t.Fatalf("deliveryAttemptDiagnostics returned error: %v", err)
	}
	if len(diagnostics.Sender) != 320 || len(diagnostics.EnhancedStatus) != 64 || len(diagnostics.DSNEnvelopeID) != 100 || len(diagnostics.OriginalRecipient) != 500 {
		t.Fatalf("diagnostics not bounded: %+v", diagnostics)
	}
	if diagnostics.DSNReturn != "HDRS" || string(diagnostics.DSNNotify) != `["FAILURE","DELAY"]` {
		t.Fatalf("diagnostics DSN fields = %+v", diagnostics)
	}

	empty, err := deliveryAttemptDiagnostics(Attempt{})
	if err != nil {
		t.Fatalf("deliveryAttemptDiagnostics empty returned error: %v", err)
	}
	if string(empty.DSNNotify) != "[]" {
		t.Fatalf("empty DSNNotify = %s, want []", empty.DSNNotify)
	}
}
