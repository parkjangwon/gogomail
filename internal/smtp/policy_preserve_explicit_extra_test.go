package smtpd

import "testing"

func TestNormalizePolicyPreservesExplicitLimits(t *testing.T) {
	policy := normalizePolicy(ReceivePolicy{MaxRecipientsPerMessage: 7, MaxMessageBytes: 4096}, 2048)
	if policy.MaxRecipientsPerMessage != 7 || policy.MaxMessageBytes != 4096 {
		t.Fatalf("normalizePolicy = %+v, want explicit limits preserved", policy)
	}
}
