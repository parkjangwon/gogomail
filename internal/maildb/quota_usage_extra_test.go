package maildb

import "testing"

func TestQuotaUsageRatioGuardsZeroLimit(t *testing.T) {
	t.Parallel()

	if got := quotaUsageRatio(100, 0); got != 0 {
		t.Fatalf("quotaUsageRatio = %v, want 0", got)
	}
}

func TestQuotaUsageRatioComputesFraction(t *testing.T) {
	t.Parallel()

	if got := quotaUsageRatio(750, 1000); got != 0.75 {
		t.Fatalf("quotaUsageRatio = %v, want 0.75", got)
	}
}

func TestQuotaRemainingComputesFreeBytes(t *testing.T) {
	t.Parallel()

	if got := quotaRemaining(750, 1000); got != 250 {
		t.Fatalf("quotaRemaining = %d, want 250", got)
	}
}

func TestQuotaRemainingClampsOverLimit(t *testing.T) {
	t.Parallel()

	if got := quotaRemaining(1200, 1000); got != 0 {
		t.Fatalf("quotaRemaining = %d, want 0", got)
	}
}
