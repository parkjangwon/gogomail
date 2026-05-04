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
	if payload["schema_version"] != APIUsageSchemaV1 {
		t.Fatalf("schema_version = %v, want %q", payload["schema_version"], APIUsageSchemaV1)
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
}

func TestAPIUsagePayloadUsesProvidedEventID(t *testing.T) {
	t.Parallel()

	payload := apiUsagePayload(Event{ID: "usage-1", Timestamp: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)})
	if payload["event_id"] != "usage-1" {
		t.Fatalf("event_id = %v, want usage-1", payload["event_id"])
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
