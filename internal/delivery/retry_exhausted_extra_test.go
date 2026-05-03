package delivery

import (
	"testing"
	"time"
)

func TestRetryPolicyRejectsAttemptPastSchedule(t *testing.T) {
	policy := RetryPolicy{Delays: []time.Duration{time.Second}}
	if delay, ok := policy.NextDelay(1); ok || delay != 0 {
		t.Fatalf("NextDelay exhausted = %s, %v; want no delay", delay, ok)
	}
}
