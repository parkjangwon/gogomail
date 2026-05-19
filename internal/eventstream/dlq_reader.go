package eventstream

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// DLQEntry is a message that was moved to a dead-letter stream after exceeding
// the maximum delivery count.
type DLQEntry struct {
	ID             string          `json:"id"`
	OriginalStream string          `json:"original_stream"`
	Group          string          `json:"group"`
	Consumer       string          `json:"consumer"`
	OriginalID     string          `json:"original_id"`
	OutboxID       string          `json:"outbox_id"`
	PartitionKey   string          `json:"partition_key"`
	Payload        json.RawMessage `json:"payload"`
	Error          string          `json:"error"`
	Deliveries     int64           `json:"deliveries"`
	DeadLetteredAt time.Time       `json:"dead_lettered_at"`
}

// DLQReader reads from and deletes entries in a Redis dead-letter stream.
type DLQReader interface {
	ListDLQ(ctx context.Context, stream string, count int64) ([]DLQEntry, error)
	DeleteDLQEntry(ctx context.Context, stream string, id string) error
}

// RedisDLQReader implements DLQReader backed by a Redis client.
type RedisDLQReader struct {
	client *redis.Client
}

// NewRedisDLQReader returns a DLQReader backed by the given Redis client.
func NewRedisDLQReader(client *redis.Client) (*RedisDLQReader, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client is required")
	}
	return &RedisDLQReader{client: client}, nil
}

// ListDLQ reads up to count entries from the named dead-letter stream.
func (r *RedisDLQReader) ListDLQ(ctx context.Context, stream string, count int64) ([]DLQEntry, error) {
	stream = strings.TrimSpace(stream)
	if stream == "" {
		return nil, fmt.Errorf("stream name is required")
	}
	if count <= 0 {
		count = 100
	}
	messages, err := r.client.XRevRangeN(ctx, stream, "+", "-", count).Result()
	if err != nil {
		return nil, fmt.Errorf("read dead-letter stream %q: %w", stream, err)
	}
	entries := make([]DLQEntry, 0, len(messages))
	for _, msg := range messages {
		entry, err := decodeDLQEntry(msg)
		if err != nil {
			continue // skip malformed entries
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// DeleteDLQEntry removes a single entry by ID from the named dead-letter stream.
func (r *RedisDLQReader) DeleteDLQEntry(ctx context.Context, stream string, id string) error {
	stream = strings.TrimSpace(stream)
	if stream == "" {
		return fmt.Errorf("stream name is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("entry id is required")
	}
	deleted, err := r.client.XDel(ctx, stream, id).Result()
	if err != nil {
		return fmt.Errorf("delete dead-letter entry %q from %q: %w", id, stream, err)
	}
	if deleted == 0 {
		return fmt.Errorf("dead-letter entry %q not found in stream %q", id, stream)
	}
	return nil
}

func decodeDLQEntry(msg redis.XMessage) (DLQEntry, error) {
	entry := DLQEntry{ID: msg.ID}
	entry.OriginalStream, _ = dlqStringField(msg.Values, "original_stream")
	entry.Group, _ = dlqStringField(msg.Values, "group")
	entry.Consumer, _ = dlqStringField(msg.Values, "consumer")
	entry.OriginalID, _ = dlqStringField(msg.Values, "original_id")
	entry.OutboxID, _ = dlqStringField(msg.Values, "outbox_id")
	entry.PartitionKey, _ = dlqStringField(msg.Values, "partition_key")
	entry.Error, _ = dlqStringField(msg.Values, "error")

	if payload, err := dlqStringField(msg.Values, "payload"); err == nil && json.Valid([]byte(payload)) {
		entry.Payload = json.RawMessage(payload)
	}
	if deliveriesStr, err := dlqStringField(msg.Values, "deliveries"); err == nil {
		if d, err := strconv.ParseInt(deliveriesStr, 10, 64); err == nil {
			entry.Deliveries = d
		}
	}
	if deadLetteredAt, err := dlqStringField(msg.Values, "dead_lettered_at"); err == nil {
		if t, err := time.Parse(time.RFC3339Nano, deadLetteredAt); err == nil {
			entry.DeadLetteredAt = t
		}
	}
	return entry, nil
}

func dlqStringField(values map[string]any, key string) (string, error) {
	v, ok := values[key]
	if !ok {
		return "", fmt.Errorf("missing field %q", key)
	}
	switch typed := v.(type) {
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}
