package smtpd

import "testing"

func TestNormalizePolicyUsesLegacyMessageSizeWhenProvided(t *testing.T) {
	policy := normalizePolicy(ReceivePolicy{MaxRecipientsPerMessage: 1}, 2048)
	if policy.MaxMessageBytes != 2048 {
		t.Fatalf("MaxMessageBytes = %d, want legacy size", policy.MaxMessageBytes)
	}
}
