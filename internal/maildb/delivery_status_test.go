package maildb

import "testing"

func TestSummarizeDeliveryAttempts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		attempts     []DeliveryAttemptView
		wantDelivery string
		wantBounce   string
	}{
		{name: "pending without attempts", wantDelivery: "pending", wantBounce: "none"},
		{name: "delivered", attempts: []DeliveryAttemptView{{Status: "delivered"}}, wantDelivery: "delivered", wantBounce: "none"},
		{name: "retrying", attempts: []DeliveryAttemptView{{Status: "retry"}}, wantDelivery: "retrying", wantBounce: "none"},
		{name: "failed", attempts: []DeliveryAttemptView{{Status: "failed"}}, wantDelivery: "failed", wantBounce: "none"},
		{name: "failed temporary enhanced status", attempts: []DeliveryAttemptView{{Status: "failed", EnhancedStatus: "4.7.1"}}, wantDelivery: "retrying", wantBounce: "none"},
		{name: "failed permanent enhanced status", attempts: []DeliveryAttemptView{{Status: "failed", EnhancedStatus: "5.1.1"}}, wantDelivery: "failed", wantBounce: "none"},
		{name: "bounced", attempts: []DeliveryAttemptView{{Status: "bounced"}}, wantDelivery: "bounced", wantBounce: "hard"},
		{name: "partial", attempts: []DeliveryAttemptView{{Status: "delivered"}, {Status: "temporary_failure"}}, wantDelivery: "partial", wantBounce: "none"},
		{name: "partial failed temporary enhanced status", attempts: []DeliveryAttemptView{{Status: "delivered"}, {Status: "failed", EnhancedStatus: "4.2.0"}}, wantDelivery: "partial", wantBounce: "none"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotDelivery, gotBounce := summarizeDeliveryAttempts(tt.attempts)
			if gotDelivery != tt.wantDelivery || gotBounce != tt.wantBounce {
				t.Fatalf("summarizeDeliveryAttempts() = (%q, %q), want (%q, %q)", gotDelivery, gotBounce, tt.wantDelivery, tt.wantBounce)
			}
		})
	}
}
