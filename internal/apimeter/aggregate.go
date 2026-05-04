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
	EventID       string
	Day           time.Time
	Month         time.Time
	Method        string
	Route         string
	Status        int
	UserID        string
	AuthSource    string
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
	claimed, err := s.claimEvent(ctx, event)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	if event.RequestCount <= 0 {
		event.RequestCount = 1
	}
	if event.Month.IsZero() {
		event.Month = time.Date(event.Day.Year(), event.Day.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	if err := s.upsert(ctx, "api_usage_daily", "day", event.Day, event); err != nil {
		return err
	}
	if err := s.upsert(ctx, "api_usage_monthly", "month", event.Month, event); err != nil {
		return err
	}
	return nil
}

func (s PostgresAggregateStore) claimEvent(ctx context.Context, event UsageEvent) (bool, error) {
	eventID := strings.TrimSpace(event.EventID)
	if eventID == "" {
		return true, nil
	}
	eventTime := event.Day
	if eventTime.IsZero() {
		eventTime = time.Now().UTC()
	}
	const query = `
INSERT INTO api_usage_events (
  event_id,
  event_timestamp,
  method,
  route,
  status,
  user_id
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (event_id) DO NOTHING`
	result, err := s.db.ExecContext(
		ctx,
		query,
		eventID,
		eventTime,
		event.Method,
		event.Route,
		event.Status,
		event.UserID,
	)
	if err != nil {
		return false, fmt.Errorf("claim api usage event: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("inspect api usage event claim: %w", err)
	}
	return affected > 0, nil
}

func (s PostgresAggregateStore) upsert(ctx context.Context, table string, bucketColumn string, bucket time.Time, event UsageEvent) error {
	query := fmt.Sprintf(`
INSERT INTO %s (
  %s,
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
ON CONFLICT (%s, method, route, status, user_id)
DO UPDATE SET
  request_count = %[1]s.request_count + EXCLUDED.request_count,
  request_bytes = %[1]s.request_bytes + EXCLUDED.request_bytes,
  response_bytes = %[1]s.response_bytes + EXCLUDED.response_bytes,
  latency_ms_total = %[1]s.latency_ms_total + EXCLUDED.latency_ms_total,
  latency_ms_max = GREATEST(%[1]s.latency_ms_max, EXCLUDED.latency_ms_max),
  last_seen_at = GREATEST(%[1]s.last_seen_at, EXCLUDED.last_seen_at)`, table, bucketColumn, bucketColumn)
	if _, err := s.db.ExecContext(
		ctx,
		query,
		bucket,
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
		return fmt.Errorf("upsert api usage aggregate %s: %w", table, err)
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
		SchemaVersion string `json:"schema_version"`
		EventID       string `json:"event_id"`
		Method        string `json:"method"`
		Route         string `json:"route"`
		Status        int    `json:"status"`
		RequestBytes  int64  `json:"request_bytes"`
		ResponseBytes int64  `json:"response_bytes"`
		LatencyMS     int64  `json:"latency_ms"`
		Timestamp     string `json:"timestamp"`
		UserID        string `json:"user_id"`
		AuthSource    string `json:"auth_source"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return UsageEvent{}, fmt.Errorf("decode api usage event: %w", err)
	}
	if strings.TrimSpace(raw.Event) != EventAPIUsage {
		return UsageEvent{}, fmt.Errorf("unexpected api metering event %q", raw.Event)
	}
	if schemaVersion := strings.TrimSpace(raw.SchemaVersion); schemaVersion != "" && schemaVersion != APIUsageSchemaV1 {
		return UsageEvent{}, fmt.Errorf("unsupported api usage schema_version %q", schemaVersion)
	}
	timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw.Timestamp))
	if err != nil {
		return UsageEvent{}, fmt.Errorf("parse api usage timestamp: %w", err)
	}
	day := timestamp.UTC().Truncate(24 * time.Hour)
	month := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC)
	return UsageEvent{
		EventID:       strings.TrimSpace(raw.EventID),
		Day:           day,
		Month:         month,
		Method:        strings.TrimSpace(raw.Method),
		Route:         strings.TrimSpace(raw.Route),
		Status:        raw.Status,
		UserID:        strings.TrimSpace(raw.UserID),
		AuthSource:    normalizeAuthSource(raw.AuthSource),
		RequestBytes:  raw.RequestBytes,
		ResponseBytes: raw.ResponseBytes,
		LatencyMS:     raw.LatencyMS,
		RequestCount:  1,
	}, nil
}
