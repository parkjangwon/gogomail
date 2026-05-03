package delivery

import "testing"

func TestRetryPolicyRejectsEmptyDelaySchedule(t *testing.T) {
	if delay, ok := (RetryPolicy{}).NextDelay(0); ok || delay != 0 {
		t.Fatalf("NextDelay for empty policy = %s, %v; want no delay", delay, ok)
	}
}
