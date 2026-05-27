package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gogomail/gogomail/internal/ratelimit"
	"github.com/redis/go-redis/v9"
)

// adminLoginRateLimiter is satisfied by both AdminIPRateLimiter (in-memory)
// and RedisAdminLoginLimiter (distributed).
type adminLoginRateLimiter interface {
	Middleware(next http.Handler) http.Handler
}

// RedisAdminLoginLimiter wraps ratelimit.RedisFixedWindowLimiter to implement
// adminLoginRateLimiter. It shares counters across all server instances.
type RedisAdminLoginLimiter struct {
	inner *ratelimit.RedisFixedWindowLimiter
}

// NewRedisAdminLoginLimiter returns a rate limiter backed by Redis.
func NewRedisAdminLoginLimiter(client *redis.Client, limit int64, window time.Duration) *RedisAdminLoginLimiter {
	return &RedisAdminLoginLimiter{
		inner: ratelimit.NewRedisFixedWindowLimiter(client, "admin:login", limit, window),
	}
}

func (l *RedisAdminLoginLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := adminClientIP(r)
		dec, err := l.inner.Allow(r.Context(), ip)
		if err != nil {
			// Redis is unavailable — fail open so a Redis blip does not lock admins
			// out entirely. Log at warn so ops are alerted.
			slog.WarnContext(r.Context(), "admin login rate limiter: Redis error, allowing request",
				"error", err, "remote_ip", ip)
			next.ServeHTTP(w, r)
			return
		}
		if !dec.Allowed {
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
