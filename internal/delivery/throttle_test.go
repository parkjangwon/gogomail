package delivery

import (
	"context"
	"testing"

	"github.com/gogomail/gogomail/internal/outbound"
)

func TestInMemoryThrottlerLimitsFarmConcurrency(t *testing.T) {
	throttler := NewInMemoryThrottler(ThrottlePolicy{
		FarmMaxConcurrent: map[outbound.Farm]int{outbound.FarmBulk: 1},
	})
	job := Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmBulk, To: []outbound.Address{{Email: "a@example.com"}}}}

	release, err := throttler.Acquire(context.Background(), job)
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	if _, err := throttler.Acquire(context.Background(), job); err == nil {
		t.Fatal("second Acquire() error = nil, want throttle")
	}
	release()
	if release2, err := throttler.Acquire(context.Background(), job); err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	} else {
		release2()
	}
}

func TestInMemoryThrottlerLimitsRecipientDomain(t *testing.T) {
	throttler := NewInMemoryThrottler(ThrottlePolicy{
		DomainMaxConcurrent: map[string]int{"example.net": 1},
	})
	first := Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmGeneral, To: []outbound.Address{{Email: "a@example.net"}}}}
	second := Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmGeneral, To: []outbound.Address{{Email: "b@example.net"}}}}

	release, err := throttler.Acquire(context.Background(), first)
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	defer release()
	if _, err := throttler.Acquire(context.Background(), second); err == nil {
		t.Fatal("second Acquire() error = nil, want domain throttle")
	}
}

func TestInMemoryThrottlerReleaseIsIdempotent(t *testing.T) {
	throttler := NewInMemoryThrottler(ThrottlePolicy{DefaultConcurrent: 1})
	job := Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmGeneral, To: []outbound.Address{{Email: "a@example.com"}}}}

	release, err := throttler.Acquire(context.Background(), job)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	release()
	release()
	if release2, err := throttler.Acquire(context.Background(), job); err != nil {
		t.Fatalf("Acquire() after duplicate release error = %v", err)
	} else {
		release2()
	}
}
