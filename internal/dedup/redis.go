package dedup

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type RedisDeduplicator struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisDeduplicator(client *redis.Client, ttl time.Duration) *RedisDeduplicator {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &RedisDeduplicator{client: client, ttl: ttl}
}

func (d *RedisDeduplicator) CheckAndSet(ctx context.Context, key smtpd.DedupKey) (bool, error) {
	ok, err := d.client.SetNX(ctx, redisKey(key), "1", d.ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func redisKey(key smtpd.DedupKey) string {
	return "dedup:" + strings.TrimSpace(key.MessageID) + ":" + strings.ToLower(strings.TrimSpace(key.Recipient))
}
