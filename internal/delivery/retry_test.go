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

func TestRetryPolicyNextScheduledDelayAppliesDeterministicJitter(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{Delays: []time.Duration{10 * time.Minute}, JitterRatio: 0.20}
	first, ok := policy.NextScheduledDelay("msg-1", 0)
	if !ok {
		t.Fatal("NextScheduledDelay returned ok=false")
	}
	second, ok := policy.NextScheduledDelay("msg-1", 0)
	if !ok {
		t.Fatal("NextScheduledDelay returned ok=false on second call")
	}
	if first != second {
		t.Fatalf("jitter = %s then %s, want deterministic value", first, second)
	}
	if first < 8*time.Minute || first > 12*time.Minute {
		t.Fatalf("jittered delay = %s, want within ±20%% of 10m", first)
	}
}

func TestRetryPolicyNextScheduledDelayCapsMaxDelay(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{
		Delays:      []time.Duration{24 * time.Hour},
		JitterRatio: 0,
		MaxDelay:    12 * time.Hour,
	}
	delay, ok := policy.NextScheduledDelay("msg-1", 0)
	if !ok {
		t.Fatal("NextScheduledDelay returned ok=false")
	}
	if delay != 12*time.Hour {
		t.Fatalf("delay = %s, want max cap 12h", delay)
	}
}

func TestRetryPolicyNextScheduledDelayCanDisableJitter(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{Delays: []time.Duration{time.Minute}}
	delay, ok := policy.NextScheduledDelay("msg-1", 0)
	if !ok {
		t.Fatal("NextScheduledDelay returned ok=false")
	}
	if delay != time.Minute {
		t.Fatalf("delay = %s, want base delay with jitter disabled", delay)
	}
}

func TestRetryPolicyNextScheduledDelayClampsHugeJitter(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{Delays: []time.Duration{10 * time.Minute}, JitterRatio: 99}
	delay, ok := policy.NextScheduledDelay("msg-1", 0)
	if !ok {
		t.Fatal("NextScheduledDelay returned ok=false")
	}
	if delay < time.Millisecond || delay > 20*time.Minute {
		t.Fatalf("delay = %s, want clamped jitter range", delay)
	}
}

func TestRetrySchedulerReportsExhaustion(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{Delays: []time.Duration{time.Minute}}
	if _, ok := policy.NextDelay(1); ok {
		t.Fatal("NextDelay returned ok for exhausted retry")
	}
}
