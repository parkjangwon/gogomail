package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

type RedisFixedWindowLimiter struct {
	client    *redis.Client
	namespace string
	limit     int64
	window    time.Duration
}

func NewRedisFixedWindowLimiter(client *redis.Client, namespace string, limit int64, window time.Duration) *RedisFixedWindowLimiter {
	namespace = normalizeLimiterNamespace(namespace)
	if limit <= 0 {
		limit = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RedisFixedWindowLimiter{client: client, namespace: namespace, limit: limit, window: window}
}

func (l *RedisFixedWindowLimiter) Allow(ctx context.Context, key string) (Decision, error) {
	if l == nil || l.client == nil {
		return Decision{Allowed: true}, nil
	}
	redisKey := fixedWindowRedisKey(l.namespace, key)
	count, err := l.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return Decision{}, err
	}
	if count == 1 {
		if err := l.client.Expire(ctx, redisKey, l.window).Err(); err != nil {
			return Decision{}, err
		}
	}
	if count <= l.limit {
		return Decision{Allowed: true}, nil
	}
	ttl, err := l.client.TTL(ctx, redisKey).Result()
	if err != nil {
		return Decision{}, err
	}
	if ttl <= 0 {
		ttl = l.window
	}
	return Decision{Allowed: false, RetryAfter: ttl}, nil
}

func fixedWindowRedisKey(namespace string, key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return fmt.Sprintf("ratelimit:%s:%s", normalizeLimiterNamespace(namespace), hex.EncodeToString(sum[:]))
}

func normalizeLimiterNamespace(namespace string) string {
	namespace = strings.TrimSpace(strings.ToLower(namespace))
	if namespace == "" {
		return "generic"
	}
	var b strings.Builder
	for _, r := range namespace {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == ':':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 96 {
			break
		}
	}
	if b.Len() == 0 {
		return "generic"
	}
	return b.String()
}
