package audit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDeliveryStatusAuditLog(t *testing.T) {
	t.Parallel()

	log, err := DeliveryStatusAuditLog([]byte(`{
		"event":" mail.bounced ",
		"message_id":" 018f0000-0000-7000-8000-000000000001 ",
		"rfc_message_id":" <msg@example.com> ",
		"company_id":" 11111111-1111-4111-8111-111111111111 ",
		"domain_id":" 22222222-2222-4222-8222-222222222222 ",
		"farm":" general ",
		"sender":" sender@example.com ",
		"recipient":" user@example.net ",
		"recipient_domain":" example.net ",
		"status":" bounced ",
		"error_message":" 550 no such user ",
		"attempted_at":" 2026-05-03T09:00:00Z "
	}`))
	if err != nil {
		t.Fatalf("DeliveryStatusAuditLog returned error: %v", err)
	}
	if log.Action != "mail.bounced" {
		t.Fatalf("Action = %q, want mail.bounced", log.Action)
	}
	if log.Result != "failure" {
		t.Fatalf("Result = %q, want failure", log.Result)
	}
	if log.TargetID != "018f0000-0000-7000-8000-000000000001" || log.CompanyID != "11111111-1111-4111-8111-111111111111" || log.DomainID != "22222222-2222-4222-8222-222222222222" {
		t.Fatalf("ids = %q/%q/%q", log.TargetID, log.CompanyID, log.DomainID)
	}
	if !json.Valid(log.Detail) {
		t.Fatal("Detail is not valid JSON")
	}
	var detail map[string]any
	if err := json.Unmarshal(log.Detail, &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail["sender"] != "sender@example.com" {
		t.Fatalf("detail sender = %v, want sender@example.com", detail["sender"])
	}
	if detail["recipient"] != "user@example.net" || detail["farm"] != "general" || detail["status"] != "bounced" {
		t.Fatalf("detail = %#v", detail)
	}
}

func TestDeliveryStatusAuditLogAcceptsExhausted(t *testing.T) {
	t.Parallel()

	log, err := DeliveryStatusAuditLog([]byte(`{
		"event":"mail.delivery_exhausted",
		"message_id":"018f0000-0000-7000-8000-000000000001",
		"company_id":"11111111-1111-4111-8111-111111111111",
		"domain_id":"22222222-2222-4222-8222-222222222222",
		"sender":"sender@example.com",
		"status":"exhausted",
		"error_message":"retry budget exhausted"
	}`))
	if err != nil {
		t.Fatalf("DeliveryStatusAuditLog returned error: %v", err)
	}
	if log.Action != "mail.delivery_exhausted" {
		t.Fatalf("Action = %q, want mail.delivery_exhausted", log.Action)
	}
	if log.Result != "failure" {
		t.Fatalf("Result = %q, want failure", log.Result)
	}
}

func TestDeliveryStatusAuditLogRejectsInvalidMessageID(t *testing.T) {
	t.Parallel()

	_, err := DeliveryStatusAuditLog([]byte("{\"event\":\"mail.delivered\",\"message_id\":\"msg-1\\r\\nmsg-2\"}"))
	if err == nil {
		t.Fatal("DeliveryStatusAuditLog accepted invalid message_id")
	}
}

func TestDeliveryStatusAuditLogRejectsOversizedMessageID(t *testing.T) {
	t.Parallel()

	_, err := DeliveryStatusAuditLog([]byte(`{
		"event":"mail.delivered",
		"message_id":"` + strings.Repeat("m", maxDeliveryAuditMessageIDBytes+1) + `"
	}`))
	if err == nil || !strings.Contains(err.Error(), "message_id") {
		t.Fatalf("DeliveryStatusAuditLog error = %v, want oversized message_id", err)
	}
}
