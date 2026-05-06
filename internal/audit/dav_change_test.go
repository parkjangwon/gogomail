package audit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/eventstream"
)

func TestDAVChangeAuditLogCalendarChanged(t *testing.T) {
	t.Parallel()

	log, err := DAVChangeAuditLog(json.RawMessage(`{
		"event":"calendar.changed",
		"schema_version":"2026-05-06.dav-change.v1",
		"dav_kind":"caldav",
		"action":"object-upserted",
		"user_id":"11111111-1111-4111-8111-111111111111",
		"owner_user_id":"11111111-1111-4111-8111-111111111111",
		"actor_user_id":"44444444-4444-4444-8444-444444444444",
		"delegated":true,
		"collection_id":"22222222-2222-4222-8222-222222222222",
		"object_name":"event-1.ics",
		"etag":"\"etag-1\"",
		"sync_token":"sync-1",
		"changed_at":"2026-05-06T12:00:00Z"
	}`))
	if err != nil {
		t.Fatalf("DAVChangeAuditLog returned error: %v", err)
	}
	if log.Category != "dav" || log.Action != DAVChangeEventCalendar || log.TargetType != "calendar" || log.Result != "success" {
		t.Fatalf("log routing fields = %+v", log)
	}
	if log.UserID != "11111111-1111-4111-8111-111111111111" || log.ActorID != "44444444-4444-4444-8444-444444444444" || log.TargetID != "22222222-2222-4222-8222-222222222222" {
		t.Fatalf("log ids = %+v", log)
	}
	var detail map[string]any
	if err := json.Unmarshal(log.Detail, &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail["dav_kind"] != "caldav" || detail["action"] != "object-upserted" || detail["object_name"] != "event-1.ics" || detail["sync_token"] != "sync-1" {
		t.Fatalf("detail = %+v", detail)
	}
	if detail["owner_user_id"] != "11111111-1111-4111-8111-111111111111" || detail["actor_user_id"] != "44444444-4444-4444-8444-444444444444" || detail["delegated"] != true {
		t.Fatalf("delegated detail = %+v", detail)
	}
}

func TestDAVChangeAuditLogContactsChanged(t *testing.T) {
	t.Parallel()

	log, err := DAVChangeAuditLog(json.RawMessage(`{
		"event":"contacts.changed",
		"schema_version":"2026-05-06.dav-change.v1",
		"dav_kind":"carddav",
		"action":"contact-deleted",
		"user_id":"11111111-1111-4111-8111-111111111111",
		"collection_id":"33333333-3333-4333-8333-333333333333",
		"sync_token":"sync-2",
		"changed_at":"2026-05-06T12:00:00Z"
	}`))
	if err != nil {
		t.Fatalf("DAVChangeAuditLog returned error: %v", err)
	}
	if log.Action != DAVChangeEventContacts || log.TargetType != "addressbook" || log.TargetID != "33333333-3333-4333-8333-333333333333" {
		t.Fatalf("log = %+v", log)
	}
}

func TestDAVChangeAuditLogRejectsUnsupportedSchema(t *testing.T) {
	t.Parallel()

	_, err := DAVChangeAuditLog(json.RawMessage(`{
		"event":"calendar.changed",
		"schema_version":"future",
		"dav_kind":"caldav",
		"action":"object-upserted",
		"user_id":"user-1",
		"collection_id":"calendar-1",
		"sync_token":"sync-1",
		"changed_at":"2026-05-06T12:00:00Z"
	}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported DAV change audit schema_version") {
		t.Fatalf("err = %v, want unsupported schema", err)
	}
}

func TestDAVChangeAuditLogRejectsInvalidScalar(t *testing.T) {
	t.Parallel()

	_, err := DAVChangeAuditLog(json.RawMessage(`{
		"event":"contacts.changed",
		"schema_version":"2026-05-06.dav-change.v1",
		"dav_kind":"carddav",
		"action":"contact-upserted",
		"user_id":"user-1",
		"collection_id":"book-1",
		"object_name":"bad\nname.vcf",
		"sync_token":"sync-1",
		"changed_at":"2026-05-06T12:00:00Z"
	}`))
	if err == nil || !strings.Contains(err.Error(), "invalid object_name") {
		t.Fatalf("err = %v, want invalid object name", err)
	}
}

func TestDAVChangeHandlerRecordsAuditLog(t *testing.T) {
	t.Parallel()

	repository := &captureAuditRepository{}
	handler := NewDAVChangeHandler(repository)
	err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: json.RawMessage(`{
		"event":"calendar.changed",
		"schema_version":"2026-05-06.dav-change.v1",
		"dav_kind":"caldav",
		"action":"collection-updated",
		"user_id":"11111111-1111-4111-8111-111111111111",
		"collection_id":"22222222-2222-4222-8222-222222222222",
		"sync_token":"sync-1",
		"changed_at":"2026-05-06T12:00:00Z"
	}`)})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if len(repository.logs) != 1 || repository.logs[0].Action != DAVChangeEventCalendar {
		t.Fatalf("logs = %+v", repository.logs)
	}
}

type captureAuditRepository struct {
	logs []Log
}

func (r *captureAuditRepository) Insert(_ context.Context, log Log) error {
	r.logs = append(r.logs, log)
	return nil
}
