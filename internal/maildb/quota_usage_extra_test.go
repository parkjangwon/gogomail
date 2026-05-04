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
