package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

type RedisStreamPublisher struct {
	client *redis.Client
}

func NewRedisStreamPublisher(client *redis.Client) *RedisStreamPublisher {
	return &RedisStreamPublisher{client: client}
}

func (p *RedisStreamPublisher) Publish(ctx context.Context, event Event) error {
	if p.client == nil {
		return fmt.Errorf("redis client is required")
	}
	event, err := normalizeRedisStreamEvent(event)
	if err != nil {
		return err
	}

	values := map[string]any{
		"outbox_id":     event.ID,
		"partition_key": event.PartitionKey,
		"payload":       string(event.Payload),
	}

	if err := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: event.Topic,
		Values: values,
	}).Err(); err != nil {
		return fmt.Errorf("publish outbox event to redis stream %q: %w", event.Topic, err)
	}
	return nil
}

func normalizeRedisStreamEvent(event Event) (Event, error) {
	event.ID = strings.TrimSpace(event.ID)
	event.Topic = strings.TrimSpace(event.Topic)
	event.PartitionKey = strings.TrimSpace(event.PartitionKey)
	event.Payload = json.RawMessage(strings.TrimSpace(string(event.Payload)))
	if event.Topic == "" {
		return Event{}, fmt.Errorf("outbox event topic is required")
	}
	if strings.ContainsAny(event.Topic, "\r\n") {
		return Event{}, fmt.Errorf("outbox event topic is invalid")
	}
	if event.ID == "" {
		return Event{}, fmt.Errorf("outbox event id is required")
	}
	if len(event.Payload) == 0 || !json.Valid(event.Payload) {
		return Event{}, fmt.Errorf("outbox event payload must be valid json")
	}
	return event, nil
}
