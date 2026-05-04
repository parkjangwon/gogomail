package audit

import (
	"encoding/json"
	"testing"
	"time"
)

func TestComputeHashChangesWithPrevHash(t *testing.T) {
	t.Parallel()

	log := Log{
		Category:   "mail",
		Action:     "mail.received",
		TargetType: "message",
		TargetID:   "018f0000-0000-7000-8000-000000000001",
		Result:     "success",
		Detail:     json.RawMessage(`{"message_id":"msg-1"}`),
		CreatedAt:  time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
	}

	first, err := ComputeHash("", log)
	if err != nil {
		t.Fatalf("ComputeHash returned error: %v", err)
	}
	second, err := ComputeHash(first, log)
	if err != nil {
		t.Fatalf("ComputeHash returned error: %v", err)
	}
	if first == second {
		t.Fatal("hash did not change when prev_hash changed")
	}
}

func TestMailStoredAuditLog(t *testing.T) {
	t.Parallel()

	log, err := MailStoredAuditLog([]byte(`{
		"event":" mail.stored ",
		"schema_version":" 2026-05-04.mail-stored.v1 ",
		"message_id":" 018f0000-0000-7000-8000-000000000001 ",
		"rfc_message_id":" <abc@example.com> ",
		"company_id":" 11111111-1111-4111-8111-111111111111 ",
		"domain_id":" 22222222-2222-4222-8222-222222222222 ",
		"user_id":" 33333333-3333-4333-8333-333333333333 ",
		"recipient":" user@example.com ",
		"subject":" hello ",
		"storage_path":" mailstore/example.eml ",
		"received_at":" 2026-05-03T09:00:00Z ",
		"size":123
	}`))
	if err != nil {
		t.Fatalf("MailStoredAuditLog returned error: %v", err)
	}
	if log.Category != "mail" {
		t.Fatalf("Category = %q, want mail", log.Category)
	}
	if log.Action != "mail.received" {
		t.Fatalf("Action = %q, want mail.received", log.Action)
	}
	if log.TargetID != "018f0000-0000-7000-8000-000000000001" {
		t.Fatalf("TargetID = %q", log.TargetID)
	}
	if log.CompanyID != "11111111-1111-4111-8111-111111111111" || log.DomainID != "22222222-2222-4222-8222-222222222222" || log.UserID != "33333333-3333-4333-8333-333333333333" {
		t.Fatalf("tenant ids = %q/%q/%q", log.CompanyID, log.DomainID, log.UserID)
	}
	if !json.Valid(log.Detail) {
		t.Fatal("Detail is not valid JSON")
	}
	var detail map[string]any
	if err := json.Unmarshal(log.Detail, &detail); err != nil {
		t.Fatalf("detail unmarshal: %v", err)
	}
	if detail["recipient"] != "user@example.com" || detail["subject"] != "hello" || detail["storage_path"] != "mailstore/example.eml" {
		t.Fatalf("detail = %#v", detail)
	}
}

func TestMailStoredAuditLogRejectsUnsupportedSchemaVersion(t *testing.T) {
	t.Parallel()

	_, err := MailStoredAuditLog([]byte(`{
		"event":"mail.stored",
		"schema_version":"2099-01-01.mail-stored.v9",
		"message_id":"018f0000-0000-7000-8000-000000000001"
	}`))
	if err == nil {
		t.Fatal("MailStoredAuditLog accepted unsupported schema version")
	}
}

func TestMailStoredAuditLogRejectsInvalidMessageID(t *testing.T) {
	t.Parallel()

	_, err := MailStoredAuditLog([]byte("{\"event\":\"mail.stored\",\"message_id\":\"msg-1\\nmsg-2\"}"))
	if err == nil {
		t.Fatal("MailStoredAuditLog accepted invalid message_id")
	}
}
