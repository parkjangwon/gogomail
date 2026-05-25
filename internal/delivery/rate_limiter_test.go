package delivery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

// makeRateLimitJob creates a job with a single recipient at the given address.
func makeRateLimitJob(email string) Job {
	return Job{QueuedMessage: QueuedMessage{To: []outbound.Address{{Email: email}}}}
}

// --- InMemoryDomainRateLimiter tests ---

func TestInMemoryRateLimiterAllowsUnderLimit(t *testing.T) {
	policy := DomainRateLimitPolicy{DefaultMessagesPerMinute: 5}
	rl := NewInMemoryDomainRateLimiter(policy)

	job := makeRateLimitJob("user@gmail.com")
	for i := 0; i < 5; i++ {
		if err := rl.Allow(context.Background(), job); err != nil {
			t.Fatalf("Allow call %d: unexpected error: %v", i+1, err)
		}
	}
}

func TestInMemoryRateLimiterBlocksOverLimit(t *testing.T) {
	policy := DomainRateLimitPolicy{DefaultMessagesPerMinute: 2}
	rl := NewInMemoryDomainRateLimiter(policy)

	job := makeRateLimitJob("user@gmail.com")
	_ = rl.Allow(context.Background(), job)
	_ = rl.Allow(context.Background(), job)
	err := rl.Allow(context.Background(), job)
	if err == nil {
		t.Fatal("expected RateLimitError, got nil")
	}
	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("err type = %T, want *RateLimitError", err)
	}
	if rle.Domain != "gmail.com" {
		t.Fatalf("domain = %q, want gmail.com", rle.Domain)
	}
	if rle.LimitPerMinute != 2 {
		t.Fatalf("limit = %d, want 2", rle.LimitPerMinute)
	}
}

func TestInMemoryRateLimiterPerDomainPolicyOverridesDefault(t *testing.T) {
	policy := DomainRateLimitPolicy{
		DefaultMessagesPerMinute: 10,
		DomainMessagesPerMinute: map[string]int{
			"hotmail.com": 1,
		},
	}
	rl := NewInMemoryDomainRateLimiter(policy)

	hotmail := makeRateLimitJob("user@hotmail.com")
	gmail := makeRateLimitJob("user@gmail.com")

	// hotmail.com is limited to 1/min → second call must fail.
	if err := rl.Allow(context.Background(), hotmail); err != nil {
		t.Fatalf("first hotmail call: %v", err)
	}
	if err := rl.Allow(context.Background(), hotmail); err == nil {
		t.Fatal("second hotmail call: expected error, got nil")
	}

	// gmail.com uses default (10) → should still succeed.
	for i := 0; i < 5; i++ {
		if err := rl.Allow(context.Background(), gmail); err != nil {
			t.Fatalf("gmail call %d: unexpected error: %v", i+1, err)
		}
	}
}

func TestInMemoryRateLimiterZeroDefaultMeansUnlimited(t *testing.T) {
	policy := DomainRateLimitPolicy{DefaultMessagesPerMinute: 0}
	rl := NewInMemoryDomainRateLimiter(policy)

	job := makeRateLimitJob("user@example.com")
	for i := 0; i < 100; i++ {
		if err := rl.Allow(context.Background(), job); err != nil {
			t.Fatalf("Allow call %d: unexpected error: %v", i+1, err)
		}
	}
}

func TestInMemoryRateLimiterWindowResetsAfterExpiry(t *testing.T) {
	policy := DomainRateLimitPolicy{DefaultMessagesPerMinute: 1}
	clock := &fakeClock{now: time.Now()}
	rl := newInMemoryDomainRateLimiterWithClock(policy, clock.Now)

	job := makeRateLimitJob("user@example.com")
	if err := rl.Allow(context.Background(), job); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := rl.Allow(context.Background(), job); err == nil {
		t.Fatal("second call in same window: expected error, got nil")
	}

	// Advance clock by 1 minute + 1 second → new window.
	clock.now = clock.now.Add(61 * time.Second)
	if err := rl.Allow(context.Background(), job); err != nil {
		t.Fatalf("first call in new window: %v", err)
	}
}

func TestInMemoryRateLimiterMultipleRecipientDomainsCheckedIndependently(t *testing.T) {
	// A job with recipients at two different domains that have different limits.
	policy := DomainRateLimitPolicy{
		DomainMessagesPerMinute: map[string]int{
			"gmail.com":   100,
			"hotmail.com": 1,
		},
	}
	rl := NewInMemoryDomainRateLimiter(policy)

	// Mix of gmail and hotmail recipients.
	multi := Job{QueuedMessage: QueuedMessage{To: []outbound.Address{
		{Email: "a@gmail.com"},
		{Email: "b@hotmail.com"},
	}}}

	// First call: both under limit → OK.
	if err := rl.Allow(context.Background(), multi); err != nil {
		t.Fatalf("first call: %v", err)
	}
	// Second call: hotmail.com is now at limit → should be blocked.
	if err := rl.Allow(context.Background(), multi); err == nil {
		t.Fatal("second call: expected RateLimitError, got nil")
	}
}

// --- RateLimitError tests ---

func TestRateLimitErrorMessage(t *testing.T) {
	err := &RateLimitError{Domain: "gmail.com", LimitPerMinute: 60}
	want := "rate limit exceeded for domain gmail.com (60/min)"
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}

// --- fakeClock helper ---

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }
