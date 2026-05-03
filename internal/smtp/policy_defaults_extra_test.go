package smtpd

import "testing"

func TestNormalizePolicyAppliesRecipientDefault(t *testing.T) {
	policy := normalizePolicy(ReceivePolicy{MaxMessageBytes: 1024}, 0)
	if policy.MaxRecipientsPerMessage != defaultMaxRecipientsPerMessage {
		t.Fatalf("MaxRecipientsPerMessage = %d, want default", policy.MaxRecipientsPerMessage)
	}
}
