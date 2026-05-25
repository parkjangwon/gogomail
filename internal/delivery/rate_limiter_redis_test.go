package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// fakeRedisRateLimiter is a minimal in-memory fake for testing RedisDomainRateLimiter.
// It emulates the two Lua scripts used by the limiter:
//   - GET script (currentCount): returns the stored int64 or 0.
//   - INCR script (increment): increments and returns the new int64.
//
// Distinction: the GET script has no args; the INCR script has one arg ("75").
type fakeRedisEval struct {
	counts  map[string]int64
	evalErr error
}

func newFakeRedisEval() *fakeRedisEval {
	return &fakeRedisEval{counts: make(map[string]int64)}
}

func (f *fakeRedisEval) Eval(_ context.Context, _ string, keys []string, args ...interface{}) *redis.Cmd {
	if f.evalErr != nil {
		return redis.NewCmdResult(nil, f.evalErr)
	}
	key := keys[0]
	if len(args) == 0 {
		// GET script: return current count.
		return redis.NewCmdResult(f.counts[key], nil)
	}
	// INCR script: increment and return new count.
	f.counts[key]++
	return redis.NewCmdResult(f.counts[key], nil)
}

func TestRedisDomainRateLimiterAllowsUnderLimit(t *testing.T) {
	fake := newFakeRedisEval()
	policy := DomainRateLimitPolicy{DefaultMessagesPerMinute: 5}
	rl := newRedisDomainRateLimiterWithClock(fake, "test", policy, time.Now)
	job := makeRateLimitJob("a@gmail.com")
	for i := 0; i < 5; i++ {
		if err := rl.Allow(context.Background(), job); err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
	}
}

func TestRedisDomainRateLimiterBlocksOverLimit(t *testing.T) {
	fake := newFakeRedisEval()
	policy := DomainRateLimitPolicy{DefaultMessagesPerMinute: 3}
	rl := newRedisDomainRateLimiterWithClock(fake, "test", policy, time.Now)
	job := makeRateLimitJob("a@gmail.com")

	for i := 0; i < 3; i++ {
		if err := rl.Allow(context.Background(), job); err != nil {
			t.Fatalf("pre-fill %d: %v", i, err)
		}
	}
	// Counter is now 3 == limit; next Allow should be blocked.
	if err := rl.Allow(context.Background(), job); err == nil {
		t.Fatal("expected RateLimitError, got nil")
	}
}

func TestRedisDomainRateLimiterPerDomainOverridesDefault(t *testing.T) {
	fake := newFakeRedisEval()
	policy := DomainRateLimitPolicy{
		DefaultMessagesPerMinute: 10,
		DomainMessagesPerMinute:  map[string]int{"yahoo.com": 2},
	}
	rl := newRedisDomainRateLimiterWithClock(fake, "test", policy, time.Now)

	yahooJob := makeRateLimitJob("u@yahoo.com")
	for i := 0; i < 2; i++ {
		if err := rl.Allow(context.Background(), yahooJob); err != nil {
			t.Fatalf("yahoo %d: %v", i, err)
		}
	}
	if err := rl.Allow(context.Background(), yahooJob); err == nil {
		t.Fatal("expected yahoo rate limit")
	}

	// gmail still under default limit.
	if err := rl.Allow(context.Background(), makeRateLimitJob("u@gmail.com")); err != nil {
		t.Fatalf("gmail should not be limited: %v", err)
	}
}

func TestRedisDomainRateLimiterFailsOpenOnError(t *testing.T) {
	fake := newFakeRedisEval()
	fake.evalErr = redis.Nil
	policy := DomainRateLimitPolicy{DefaultMessagesPerMinute: 1}
	rl := newRedisDomainRateLimiterWithClock(fake, "test", policy, time.Now)
	job := makeRateLimitJob("a@gmail.com")

	// Redis error → fail open (allow delivery).
	if err := rl.Allow(context.Background(), job); err != nil {
		t.Fatalf("expected fail-open on Redis error, got: %v", err)
	}
}

func TestRedisDomainRateLimiterWindowResetsAfterMinute(t *testing.T) {
	fake := newFakeRedisEval()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	policy := DomainRateLimitPolicy{DefaultMessagesPerMinute: 2}
	rl := newRedisDomainRateLimiterWithClock(fake, "test", policy, func() time.Time { return now })
	job := makeRateLimitJob("a@gmail.com")

	_ = rl.Allow(context.Background(), job)
	_ = rl.Allow(context.Background(), job)
	if err := rl.Allow(context.Background(), job); err == nil {
		t.Fatal("expected rate limit in current window")
	}

	// New minute window → different key → counter starts at 0.
	now = now.Add(time.Minute)
	if err := rl.Allow(context.Background(), job); err != nil {
		t.Fatalf("expected allow in new window: %v", err)
	}
}
