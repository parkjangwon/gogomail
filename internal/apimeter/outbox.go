package apimeter

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	OutboxTopicAPIUsage = "api.event"
	EventAPIUsage       = "api.usage"
)

type SQLExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type PostgresOutboxSink struct {
	db SQLExecer
}

func NewPostgresOutboxSink(db SQLExecer) PostgresOutboxSink {
	return PostgresOutboxSink{db: db}
}

func (s PostgresOutboxSink) Record(ctx context.Context, event Event) error {
	if s.db == nil {
		return fmt.Errorf("database handle is required")
	}
	payload, err := json.Marshal(apiUsagePayload(event))
	if err != nil {
		return fmt.Errorf("marshal api usage event: %w", err)
	}
	partitionKey := strings.TrimSpace(event.UserID)
	if partitionKey == "" {
		partitionKey = strings.TrimSpace(event.RoutePattern)
	}
	if partitionKey == "" {
		partitionKey = "anonymous"
	}

	const query = `
INSERT INTO outbox (topic, partition_key, payload, status)
VALUES ($1, $2, $3, 'pending')`
	if _, err := s.db.ExecContext(ctx, query, OutboxTopicAPIUsage, partitionKey, payload); err != nil {
		return fmt.Errorf("insert api usage outbox event: %w", err)
	}
	return nil
}

func apiUsagePayload(event Event) map[string]any {
	timestamp := event.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	return map[string]any{
		"event":          EventAPIUsage,
		"method":         strings.TrimSpace(event.Method),
		"route":          strings.TrimSpace(event.RoutePattern),
		"status":         event.Status,
		"request_bytes":  event.RequestBytes,
		"response_bytes": event.ResponseBytes,
		"latency_ms":     event.Latency.Milliseconds(),
		"timestamp":      timestamp.UTC().Format(time.RFC3339Nano),
		"user_id":        strings.TrimSpace(event.UserID),
	}
}
