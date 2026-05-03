package backpressure

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"
)

const DefaultStateKey = "backpressure:smtp:state"

type RedisBackpressure struct {
	client *redis.Client
	key    string
}

func NewRedisBackpressure(client *redis.Client, key string) *RedisBackpressure {
	if strings.TrimSpace(key) == "" {
		key = DefaultStateKey
	}
	return &RedisBackpressure{client: client, key: key}
}

func (b *RedisBackpressure) Accept(ctx context.Context) (bool, error) {
	state, err := b.client.Get(ctx, b.key).Result()
	if err == redis.Nil {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return acceptsState(state), nil
}

func acceptsState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "", "normal", "warning":
		return true
	case "danger", "critical":
		return false
	default:
		return true
	}
}
