package apimeter

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/eventstream"
)

func TestDecodeUsageEventNormalizesDailyBucket(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(`{
		"event":"api.usage",
		"schema_version":"2026-05-04.api-usage.v1",
		"event_id":"usage-1",
		"method":"GET",
		"route":"GET /api/v1/messages",
		"status":200,
		"request_bytes":12,
		"response_bytes":34,
		"latency_ms":25,
		"timestamp":"2026-05-04T15:30:00+09:00",
		"user_id":"user-1"
	}`)
	event, err := DecodeUsageEvent(payload)
	if err != nil {
		t.Fatalf("DecodeUsageEvent returned error: %v", err)
	}

	wantDay := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	if !event.Day.Equal(wantDay) {
		t.Fatalf("Day = %s, want %s", event.Day, wantDay)
	}
	wantMonth := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if !event.Month.Equal(wantMonth) {
		t.Fatalf("Month = %s, want %s", event.Month, wantMonth)
	}
	if event.Route != "GET /api/v1/messages" || event.UserID != "user-1" {
		t.Fatalf("event = %+v", event)
	}
	if event.EventID != "usage-1" {
		t.Fatalf("EventID = %q, want usage-1", event.EventID)
	}
}

func TestDecodeUsageEventRejectsUnsupportedSchemaVersion(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(`{
		"event":"api.usage",
		"schema_version":"2099-01-01.api-usage.v9",
		"method":"GET",
		"route":"GET /api/v1/messages",
		"status":200,
		"timestamp":"2026-05-04T00:00:00Z"
	}`)
	if _, err := DecodeUsageEvent(payload); err == nil {
		t.Fatal("DecodeUsageEvent accepted unsupported schema version")
	}
}

func TestUsageHandlerAggregatesAPIUsageEvent(t *testing.T) {
	t.Parallel()

	store := &fakeUsageAggregateStore{}
	handler := NewUsageHandler(store)
	payload := json.RawMessage(`{
		"event":"api.usage",
		"method":"POST",
		"route":"POST /api/v1/messages/send",
		"status":202,
		"request_bytes":100,
		"response_bytes":50,
		"latency_ms":75,
		"timestamp":"2026-05-04T00:00:00Z",
		"user_id":"user-1"
	}`)

	err := handler.HandleEvent(context.Background(), eventstream.Message{Payload: payload})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
	if store.event.Route != "POST /api/v1/messages/send" || store.event.RequestCount != 1 {
		t.Fatalf("stored event = %+v", store.event)
	}
}

func TestPostgresAggregateStoreUpsertsDailyUsage(t *testing.T) {
	t.Parallel()

	db := &fakeUsageSQL{}
	store := NewPostgresAggregateStore(db)
	err := store.AddUsage(context.Background(), UsageEvent{
		Day:           time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Method:        "GET",
		Route:         "GET /api/v1/messages",
		Status:        200,
		UserID:        "user-1",
		RequestBytes:  12,
		ResponseBytes: 34,
		LatencyMS:     25,
		RequestCount:  1,
	})
	if err != nil {
		t.Fatalf("AddUsage returned error: %v", err)
	}
	if len(db.queries) == 0 || !strings.Contains(db.queries[0], "INSERT INTO api_usage_daily") {
		t.Fatalf("queries = %+v, want api_usage_daily upsert", db.queries)
	}
	if db.argSets[0][1] != "GET" || db.argSets[0][2] != "GET /api/v1/messages" {
		t.Fatalf("args = %+v", db.argSets[0])
	}
}

func TestPostgresAggregateStoreUpsertsMonthlyUsage(t *testing.T) {
	t.Parallel()

	db := &fakeUsageSQL{}
	store := NewPostgresAggregateStore(db)
	err := store.AddUsage(context.Background(), UsageEvent{
		Day:           time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Month:         time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Method:        "GET",
		Route:         "GET /api/v1/messages",
		Status:        200,
		UserID:        "user-1",
		RequestBytes:  12,
		ResponseBytes: 34,
		LatencyMS:     25,
		RequestCount:  1,
	})
	if err != nil {
		t.Fatalf("AddUsage returned error: %v", err)
	}
	if len(db.queries) != 2 || !strings.Contains(db.queries[1], "INSERT INTO api_usage_monthly") {
		t.Fatalf("queries = %+v, want monthly upsert after daily", db.queries)
	}
	if db.argSets[1][0] != time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("monthly args = %+v", db.argSets[1])
	}
}

func TestPostgresAggregateStoreSkipsDuplicateEventID(t *testing.T) {
	t.Parallel()

	db := &fakeUsageSQL{rowsAffected: []int64{0}}
	store := NewPostgresAggregateStore(db)
	err := store.AddUsage(context.Background(), UsageEvent{
		EventID:       "usage-1",
		Day:           time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Method:        "GET",
		Route:         "GET /api/v1/messages",
		Status:        200,
		UserID:        "user-1",
		RequestBytes:  12,
		ResponseBytes: 34,
		LatencyMS:     25,
		RequestCount:  1,
	})
	if err != nil {
		t.Fatalf("AddUsage returned error: %v", err)
	}
	if len(db.queries) != 1 || !strings.Contains(db.queries[0], "INSERT INTO api_usage_events") {
		t.Fatalf("queries = %+v, want only idempotency insert", db.queries)
	}
}

type fakeUsageAggregateStore struct {
	event UsageEvent
}

func (s *fakeUsageAggregateStore) AddUsage(_ context.Context, event UsageEvent) error {
	s.event = event
	return nil
}

type fakeUsageSQL struct {
	query        string
	args         []any
	queries      []string
	argSets      [][]any
	rowsAffected []int64
}

func (f *fakeUsageSQL) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.query = query
	f.args = args
	f.queries = append(f.queries, query)
	f.argSets = append(f.argSets, args)
	rowsAffected := int64(1)
	if len(f.rowsAffected) > 0 {
		rowsAffected = f.rowsAffected[0]
		f.rowsAffected = f.rowsAffected[1:]
	}
	return fakeUsageSQLResult{rowsAffected: rowsAffected}, nil
}

type fakeUsageSQLResult struct {
	rowsAffected int64
}

func (r fakeUsageSQLResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeUsageSQLResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }
