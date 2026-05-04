package mailservice

import "testing"

func TestNormalizeSendTextResultFillsLifecycleStatuses(t *testing.T) {
	t.Parallel()

	result := NormalizeSendTextResult(SendTextResult{ID: "msg-1"})
	if result.SendStatus != "queued" || result.DeliveryStatus != "pending" || result.BounceStatus != "none" {
		t.Fatalf("NormalizeSendTextResult = %+v", result)
	}
}
