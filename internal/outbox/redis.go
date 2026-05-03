package outbox

import (
	"context"
	"fmt"

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
	if event.Topic == "" {
		return fmt.Errorf("outbox event topic is required")
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
