package delivery

import (
	"testing"
	"time"
)

func TestRetryPolicyTreatsNegativeAttemptAsFirstAttempt(t *testing.T) {
	policy := RetryPolicy{Delays: []time.Duration{time.Minute}}
	got, ok := policy.NextDelay(-10)
	if !ok || got != time.Minute {
		t.Fatalf("NextDelay(-10) = %s, %v; want first delay", got, ok)
	}
}
