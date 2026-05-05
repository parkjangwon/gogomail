package ratelimit

import (
	"context"
	"net/netip"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type RedisLimiter struct {
	client *redis.Client
	limit  int64
	window time.Duration
}

func NewRedisLimiter(client *redis.Client, limit int64, window time.Duration) *RedisLimiter {
	if limit <= 0 {
		limit = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RedisLimiter{client: client, limit: limit, window: window}
}

func (l *RedisLimiter) Allow(ctx context.Context, key smtpd.RateLimitKey) (bool, error) {
	redisKey := redisKey(key)
	count, err := l.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		if err := l.client.Expire(ctx, redisKey, l.window).Err(); err != nil {
			return false, err
		}
	}
	return count <= l.limit, nil
}

func redisKey(key smtpd.RateLimitKey) string {
	return "ratelimit:" + string(key.Stage) + ":" + rateLimitRemoteBucket(key.RemoteAddr)
}

func rateLimitRemoteBucket(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return "unknown"
	}
	if addr, err := netip.ParseAddr(remoteAddr); err == nil {
		return addr.Unmap().String()
	}
	addrPort, err := netip.ParseAddrPort(remoteAddr)
	if err != nil {
		return "unknown"
	}
	return addrPort.Addr().Unmap().String()
}
