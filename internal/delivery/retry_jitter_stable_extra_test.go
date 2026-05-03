package delivery

import (
	"testing"
	"time"
)

func TestRetryPolicyJitterIsStableForSameKeyAndAttempt(t *testing.T) {
	policy := RetryPolicy{Delays: []time.Duration{time.Minute}, JitterRatio: 0.2}
	a, okA := policy.NextScheduledDelay("message-1", 0)
	b, okB := policy.NextScheduledDelay("message-1", 0)
	if !okA || !okB || a != b {
		t.Fatalf("stable jitter delays = %s/%s ok=%v/%v, want equal", a, b, okA, okB)
	}
}
