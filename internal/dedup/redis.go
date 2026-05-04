package dedup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	return "dedup:v2:" + redisKeyHash(strings.TrimSpace(key.MessageID)) + ":" + redisKeyHash(strings.ToLower(strings.TrimSpace(key.Recipient)))
}

func redisKeyHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
