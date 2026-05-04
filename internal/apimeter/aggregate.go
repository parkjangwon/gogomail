package apimeter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/eventstream"
)

type UsageEvent struct {
	Day           time.Time
	Method        string
	Route         string
	Status        int
	UserID        string
	RequestBytes  int64
	ResponseBytes int64
	LatencyMS     int64
	RequestCount  int64
}

type UsageAggregateStore interface {
	AddUsage(ctx context.Context, event UsageEvent) error
}

type PostgresAggregateStore struct {
	db SQLExecer
}

func NewPostgresAggregateStore(db SQLExecer) PostgresAggregateStore {
	return PostgresAggregateStore{db: db}
}

func (s PostgresAggregateStore) AddUsage(ctx context.Context, event UsageEvent) error {
	if s.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if event.RequestCount <= 0 {
		event.RequestCount = 1
	}
	const query = `
INSERT INTO api_usage_daily (
  day,
  method,
  route,
  status,
  user_id,
  request_count,
  request_bytes,
  response_bytes,
  latency_ms_total,
  latency_ms_max,
  first_seen_at,
  last_seen_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now(), now())
ON CONFLICT (day, method, route, status, user_id)
DO UPDATE SET
  request_count = api_usage_daily.request_count + EXCLUDED.request_count,
  request_bytes = api_usage_daily.request_bytes + EXCLUDED.request_bytes,
  response_bytes = api_usage_daily.response_bytes + EXCLUDED.response_bytes,
  latency_ms_total = api_usage_daily.latency_ms_total + EXCLUDED.latency_ms_total,
  latency_ms_max = GREATEST(api_usage_daily.latency_ms_max, EXCLUDED.latency_ms_max),
  last_seen_at = GREATEST(api_usage_daily.last_seen_at, EXCLUDED.last_seen_at)`
	if _, err := s.db.ExecContext(
		ctx,
		query,
		event.Day,
		event.Method,
		event.Route,
		event.Status,
		event.UserID,
		event.RequestCount,
		event.RequestBytes,
		event.ResponseBytes,
		event.LatencyMS,
		event.LatencyMS,
	); err != nil {
		return fmt.Errorf("upsert api usage aggregate: %w", err)
	}
	return nil
}

type UsageHandler struct {
	store UsageAggregateStore
}

func NewUsageHandler(store UsageAggregateStore) UsageHandler {
	return UsageHandler{store: store}
}

func (h UsageHandler) HandleEvent(ctx context.Context, msg eventstream.Message) error {
	if h.store == nil {
		return fmt.Errorf("api usage aggregate store is required")
	}
	event, err := DecodeUsageEvent(msg.Payload)
	if err != nil {
		return err
	}
	return h.store.AddUsage(ctx, event)
}

func DecodeUsageEvent(payload json.RawMessage) (UsageEvent, error) {
	var raw struct {
		Event         string `json:"event"`
		Method        string `json:"method"`
		Route         string `json:"route"`
		Status        int    `json:"status"`
		RequestBytes  int64  `json:"request_bytes"`
		ResponseBytes int64  `json:"response_bytes"`
		LatencyMS     int64  `json:"latency_ms"`
		Timestamp     string `json:"timestamp"`
		UserID        string `json:"user_id"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return UsageEvent{}, fmt.Errorf("decode api usage event: %w", err)
	}
	if strings.TrimSpace(raw.Event) != EventAPIUsage {
		return UsageEvent{}, fmt.Errorf("unexpected api metering event %q", raw.Event)
	}
	timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw.Timestamp))
	if err != nil {
		return UsageEvent{}, fmt.Errorf("parse api usage timestamp: %w", err)
	}
	day := timestamp.UTC().Truncate(24 * time.Hour)
	return UsageEvent{
		Day:           day,
		Method:        strings.TrimSpace(raw.Method),
		Route:         strings.TrimSpace(raw.Route),
		Status:        raw.Status,
		UserID:        strings.TrimSpace(raw.UserID),
		RequestBytes:  raw.RequestBytes,
		ResponseBytes: raw.ResponseBytes,
		LatencyMS:     raw.LatencyMS,
		RequestCount:  1,
	}, nil
}
