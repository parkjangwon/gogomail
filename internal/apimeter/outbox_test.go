package apimeter

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPostgresOutboxSinkWritesAPIUsageEvent(t *testing.T) {
	t.Parallel()

	db := &fakeSQLExecer{}
	sink := NewPostgresOutboxSink(db)
	err := sink.Record(context.Background(), Event{
		Method:        "GET",
		RoutePattern:  "GET /api/v1/messages",
		Status:        200,
		RequestBytes:  12,
		ResponseBytes: 34,
		Latency:       25 * time.Millisecond,
		Timestamp:     time.Date(2026, 5, 4, 1, 2, 3, 4, time.UTC),
		UserID:        "user-1",
		AuthSource:    "bearer",
		Identity: Identity{
			TenantID:   "tenant-1",
			CompanyID:  "company-1",
			DomainID:   "domain-1",
			UserID:     "user-1",
			APIKeyID:   "api-key-1",
			AuthSource: "bearer",
		},
	})
	if err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	if !strings.Contains(db.query, "INSERT INTO outbox") {
		t.Fatalf("query = %q, want outbox insert", db.query)
	}
	if db.args[0] != OutboxTopicAPIUsage {
		t.Fatalf("topic = %v, want %q", db.args[0], OutboxTopicAPIUsage)
	}
	if db.args[1] != "user-1" {
		t.Fatalf("partition key = %v, want user-1", db.args[1])
	}

	var payload map[string]any
	if err := json.Unmarshal(db.args[2].([]byte), &payload); err != nil {
		t.Fatalf("payload unmarshal returned error: %v", err)
	}
	if payload["event"] != EventAPIUsage {
		t.Fatalf("event = %v, want %q", payload["event"], EventAPIUsage)
	}
	if payload["schema_version"] != APIUsageSchemaCurrent {
		t.Fatalf("schema_version = %v, want %q", payload["schema_version"], APIUsageSchemaCurrent)
	}
	if payload["event_id"] == "" {
		t.Fatal("event_id is empty")
	}
	if payload["route"] != "GET /api/v1/messages" {
		t.Fatalf("route = %v", payload["route"])
	}
	if payload["latency_ms"].(float64) != 25 {
		t.Fatalf("latency_ms = %v, want 25", payload["latency_ms"])
	}
	if payload["auth_source"] != "bearer" {
		t.Fatalf("auth_source = %v, want bearer", payload["auth_source"])
	}
	if payload["tenant_id"] != "tenant-1" || payload["company_id"] != "company-1" || payload["domain_id"] != "domain-1" {
		t.Fatalf("identity dimensions = %+v", payload)
	}
	if payload["api_key_id"] != "api-key-1" || payload["principal_id"] != "user-1" {
		t.Fatalf("identity principals = %+v", payload)
	}
}

func TestAPIUsagePayloadFallsBackToLegacyEventIdentityFields(t *testing.T) {
	t.Parallel()

	payload, err := apiUsagePayload(Event{
		Timestamp:    time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Method:       "GET",
		RoutePattern: "GET /api/v1/messages",
		UserID:       "user-1",
		AuthSource:   AuthSourceQueryUserID,
	})
	if err != nil {
		t.Fatalf("apiUsagePayload returned error: %v", err)
	}
	if payload["user_id"] != "user-1" || payload["principal_id"] != "user-1" {
		t.Fatalf("payload identity = %+v", payload)
	}
	if payload["auth_source"] != AuthSourceQueryUserID {
		t.Fatalf("auth_source = %v", payload["auth_source"])
	}
}

func TestAPIUsagePayloadClampsNegativeMetrics(t *testing.T) {
	t.Parallel()

	payload, err := apiUsagePayload(Event{
		Timestamp:     time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Method:        "GET",
		RoutePattern:  "GET /api/v1/messages",
		Status:        200,
		RequestBytes:  -1,
		ResponseBytes: -2,
		Latency:       -time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apiUsagePayload returned error: %v", err)
	}
	if payload["request_bytes"] != int64(0) || payload["response_bytes"] != int64(0) || payload["latency_ms"] != int64(0) {
		t.Fatalf("payload metrics = %+v", payload)
	}
}

func TestAPIUsageEventIDIncludesIdentityDimensions(t *testing.T) {
	t.Parallel()

	base := Event{
		Method:        "GET",
		RoutePattern:  "GET /api/v1/messages",
		Status:        200,
		RequestBytes:  1,
		ResponseBytes: 2,
		Latency:       time.Millisecond,
		Timestamp:     time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Identity:      Identity{TenantID: "tenant-1", UserID: "user-1", AuthSource: AuthSourceBearer},
	}
	firstPayload, err := apiUsagePayload(base)
	if err != nil {
		t.Fatalf("apiUsagePayload returned error: %v", err)
	}
	base.Identity.TenantID = "tenant-2"
	secondPayload, err := apiUsagePayload(base)
	if err != nil {
		t.Fatalf("apiUsagePayload returned error: %v", err)
	}
	first := firstPayload["event_id"]
	second := secondPayload["event_id"]
	if first == second {
		t.Fatalf("event_id did not change when tenant changed: %v", first)
	}
}

func TestAPIUsagePayloadUsesProvidedEventID(t *testing.T) {
	t.Parallel()

	payload, err := apiUsagePayload(Event{
		ID:           "usage-1",
		Timestamp:    time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Method:       "GET",
		RoutePattern: "GET /api/v1/messages",
	})
	if err != nil {
		t.Fatalf("apiUsagePayload returned error: %v", err)
	}
	if payload["event_id"] != "usage-1" {
		t.Fatalf("event_id = %v, want usage-1", payload["event_id"])
	}
}

func TestPostgresOutboxSinkRejectsInvalidUsageDimensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event Event
	}{
		{
			name: "method line break",
			event: Event{
				Method:       "GET\r\nX-Bad: 1",
				RoutePattern: "GET /api/v1/messages",
				Status:       200,
			},
		},
		{
			name: "route line break",
			event: Event{
				Method:       "GET",
				RoutePattern: "GET /api/v1/messages\nX-Bad: 1",
				Status:       200,
			},
		},
		{
			name: "event id line break",
			event: Event{
				ID:           "usage-1\r\nX-Bad: 1",
				Method:       "GET",
				RoutePattern: "GET /api/v1/messages",
				Status:       200,
			},
		},
		{
			name: "identity line break",
			event: Event{
				Method:       "GET",
				RoutePattern: "GET /api/v1/messages",
				Status:       200,
				Identity: Identity{
					TenantID: "tenant-1\nX-Bad: 1",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db := &fakeSQLExecer{}
			err := NewPostgresOutboxSink(db).Record(context.Background(), tc.event)
			if err == nil {
				t.Fatal("Record accepted invalid usage dimensions")
			}
			if db.query != "" {
				t.Fatalf("query = %q, want no insert", db.query)
			}
		})
	}
}

func TestPostgresOutboxSinkRequiresDB(t *testing.T) {
	t.Parallel()

	err := NewPostgresOutboxSink(nil).Record(context.Background(), Event{})
	if err == nil || !strings.Contains(err.Error(), "database handle") {
		t.Fatalf("error = %v, want database handle error", err)
	}
}

type fakeSQLExecer struct {
	query string
	args  []any
}

func (e *fakeSQLExecer) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	e.query = query
	e.args = args
	return fakeSQLResult{}, nil
}

type fakeSQLResult struct{}

func (fakeSQLResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeSQLResult) RowsAffected() (int64, error) { return 1, nil }
