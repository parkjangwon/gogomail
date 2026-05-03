package delivery

import (
	"testing"
	"time"
)

func TestRetryPolicyNextDelay(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{Delays: []time.Duration{time.Minute, 2 * time.Minute}}
	delay, ok := policy.NextDelay(0)
	if !ok || delay != time.Minute {
		t.Fatalf("NextDelay(0) = %s, %v; want 1m, true", delay, ok)
	}
	delay, ok = policy.NextDelay(1)
	if !ok || delay != 2*time.Minute {
		t.Fatalf("NextDelay(1) = %s, %v; want 2m, true", delay, ok)
	}
	_, ok = policy.NextDelay(2)
	if ok {
		t.Fatal("NextDelay(2) returned ok after attempts exhausted")
	}
}

func TestRetrySchedulerReportsExhaustion(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{Delays: []time.Duration{time.Minute}}
	if _, ok := policy.NextDelay(1); ok {
		t.Fatal("NextDelay returned ok for exhausted retry")
	}
}
