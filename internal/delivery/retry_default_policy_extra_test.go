package delivery

import (
	"testing"
	"time"
)

func TestDefaultRetryPolicyExposesOperationalBackoffWindow(t *testing.T) {
	policy := DefaultRetryPolicy()
	if len(policy.Delays) != 5 {
		t.Fatalf("default retry delays = %d, want 5", len(policy.Delays))
	}
	if policy.MaxDelay != 24*time.Hour {
		t.Fatalf("default max delay = %s, want 24h", policy.MaxDelay)
	}
}
