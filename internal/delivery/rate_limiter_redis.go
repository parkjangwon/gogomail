package delivery

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisRateLimiterClient is the Redis interface required by RedisDomainRateLimiter.
// It is satisfied by *redis.Client, *redis.ClusterClient, and test fakes.
type redisRateLimiterClient interface {
	// Eval executes a Lua script atomically.
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
}

// RedisDomainRateLimiter enforces per-domain outbound delivery rate limits
// using Redis as the shared counter store. This makes the rate limits effective
// across multiple delivery-worker replicas.
//
// Algorithm: fixed one-minute window per domain using a Redis key of the form
//
//	gogomail:delivery:rate:{domain}:{minute_unix}
//
// A Lua script atomically increments the counter and sets a 75-second TTL on the
// first write. INCR+EXPIRE in a single round-trip avoids race conditions.
type RedisDomainRateLimiter struct {
	client redisRateLimiterClient
	prefix string
	policy DomainRateLimitPolicy
	now    func() time.Time
}

// NewRedisDomainRateLimiter creates a rate limiter backed by Redis.
// prefix is used to namespace keys (e.g. "gogomail" → keys like "gogomail:delivery:rate:gmail.com:28474200").
func NewRedisDomainRateLimiter(client *redis.Client, prefix string, policy DomainRateLimitPolicy) *RedisDomainRateLimiter {
	return newRedisDomainRateLimiterWithClock(client, prefix, policy, time.Now)
}

func newRedisDomainRateLimiterWithClock(client redisRateLimiterClient, prefix string, policy DomainRateLimitPolicy, now func() time.Time) *RedisDomainRateLimiter {
	if prefix == "" {
		prefix = "gogomail"
	}
	return &RedisDomainRateLimiter{
		client: client,
		prefix: prefix,
		policy: normalizeDomainRateLimitPolicy(policy),
		now:    now,
	}
}

// incrScript atomically increments a counter and sets a 75-second TTL on first write.
// KEYS[1] = counter key
// ARGV[1] = TTL in seconds (75)
// Returns the new counter value.
const rateLimitIncrScript = `
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return count
`

// Allow checks each recipient domain in the job against configured rate limits.
//
// Two-pass approach:
//  1. For each domain, check the current count without incrementing (READ pass).
//     If any domain is already at or above its limit, return a RateLimitError.
//  2. If all domains are under their limits, increment each counter (WRITE pass).
//
// This is not fully atomic across domains, but the two-pass approach prevents
// a partial increment when one domain would be blocked. Within a single domain,
// the Lua script provides atomicity.
//
// Note: the READ pass uses the existing counter value before this request's
// increment. Using GET (not INCR) for the check pass means the counter is
// only incremented when the request is definitely allowed.
func (r *RedisDomainRateLimiter) Allow(ctx context.Context, job Job) error {
	domains := recipientDomainsForJob(job)
	if len(domains) == 0 {
		return nil
	}

	window := r.now().Truncate(time.Minute).Unix()

	// First pass: check current counts.
	for _, domain := range domains {
		limit := r.limitFor(domain)
		if limit <= 0 {
			continue
		}
		key := r.keyFor(domain, window)
		count, err := r.currentCount(ctx, key)
		if err != nil {
			// On Redis error, fail open to avoid blocking legitimate delivery.
			continue
		}
		if count >= int64(limit) {
			return &RateLimitError{Domain: domain, LimitPerMinute: limit}
		}
	}

	// Second pass: increment counters for allowed domains.
	for _, domain := range domains {
		limit := r.limitFor(domain)
		if limit <= 0 {
			continue
		}
		key := r.keyFor(domain, window)
		if err := r.increment(ctx, key); err != nil {
			// On Redis error, fail open.
			continue
		}
	}
	return nil
}

// currentCount returns the current counter value for a key (0 if key doesn't exist).
func (r *RedisDomainRateLimiter) currentCount(ctx context.Context, key string) (int64, error) {
	cmd := r.client.Eval(ctx, `
local v = redis.call('GET', KEYS[1])
if v == false then return 0 end
return tonumber(v)
`, []string{key})
	val, err := cmd.Int64()
	if err != nil {
		return 0, err
	}
	return val, nil
}

// increment atomically increments the key and sets TTL on first write.
func (r *RedisDomainRateLimiter) increment(ctx context.Context, key string) error {
	cmd := r.client.Eval(ctx, rateLimitIncrScript, []string{key}, "75")
	return cmd.Err()
}

// keyFor returns the Redis key for a domain in the given minute window.
func (r *RedisDomainRateLimiter) keyFor(domain string, minuteUnix int64) string {
	return fmt.Sprintf("%s:delivery:rate:%s:%d", r.prefix, domain, minuteUnix)
}

// limitFor returns the configured limit for a domain (0 = unlimited).
func (r *RedisDomainRateLimiter) limitFor(domain string) int {
	if r.policy.DomainMessagesPerMinute != nil {
		if limit, ok := r.policy.DomainMessagesPerMinute[domain]; ok {
			return limit
		}
	}
	return r.policy.DefaultMessagesPerMinute
}
