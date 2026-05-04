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
	if event.Route != "GET /api/v1/messages" || event.UserID != "user-1" {
		t.Fatalf("event = %+v", event)
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
	if !strings.Contains(db.query, "INSERT INTO api_usage_daily") {
		t.Fatalf("query = %q, want api_usage_daily upsert", db.query)
	}
	if db.args[1] != "GET" || db.args[2] != "GET /api/v1/messages" {
		t.Fatalf("args = %+v", db.args)
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
	query string
	args  []any
}

func (f *fakeUsageSQL) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.query = query
	f.args = args
	return fakeSQLResult{}, nil
}
