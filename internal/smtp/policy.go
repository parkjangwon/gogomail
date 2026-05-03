package smtpd

const (
	defaultMaxRecipientsPerMessage = 100
	defaultMaxMessageBytes         = 25 * 1024 * 1024
)

type ReceivePolicy struct {
	MaxRecipientsPerMessage int
	MaxMessageBytes         int64
}

func normalizePolicy(policy ReceivePolicy, legacyMaxMessageBytes int64) ReceivePolicy {
	if policy.MaxRecipientsPerMessage <= 0 {
		policy.MaxRecipientsPerMessage = defaultMaxRecipientsPerMessage
	}
	if policy.MaxMessageBytes <= 0 {
		policy.MaxMessageBytes = legacyMaxMessageBytes
	}
	if policy.MaxMessageBytes <= 0 {
		policy.MaxMessageBytes = defaultMaxMessageBytes
	}
	return policy
}
