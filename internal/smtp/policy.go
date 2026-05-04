package smtpd

import "context"

const (
	defaultMaxRecipientsPerMessage = 100
	defaultMaxMessageBytes         = 25 * 1024 * 1024
)

type ReceivePolicy struct {
	MaxRecipientsPerMessage int
	MaxMessageBytes         int64
}

// InboundDomainPolicy carries per-domain SMTP receive and submission limits.
// It is looked up once per session (on first resolved recipient for inbound,
// or from the authenticated sender's domain for submission) and overlays the
// global ReceivePolicy when the domain is in enforce mode.
type InboundDomainPolicy struct {
	InboundMode             string
	MaxRecipientsPerMessage int
	MaxMessageBytes         int64
}

// DomainPolicyLookup resolves per-domain inbound policy.  Implementations
// must be safe for concurrent calls from different sessions.
type DomainPolicyLookup interface {
	InboundDomainPolicy(ctx context.Context, domainID string) (InboundDomainPolicy, error)
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

// effectiveMaxBytes returns the more restrictive of the global limit and the
// per-domain enforce limit.  Zero means "no domain limit set".
func effectiveMaxBytes(globalLimit int64, dp *InboundDomainPolicy) int64 {
	if dp == nil || dp.InboundMode != "enforce" || dp.MaxMessageBytes <= 0 {
		return globalLimit
	}
	if dp.MaxMessageBytes < globalLimit {
		return dp.MaxMessageBytes
	}
	return globalLimit
}

// effectiveMaxRecipients returns the more restrictive recipient cap.
func effectiveMaxRecipients(globalMax int, dp *InboundDomainPolicy) int {
	if dp == nil || dp.InboundMode != "enforce" || dp.MaxRecipientsPerMessage <= 0 {
		return globalMax
	}
	if dp.MaxRecipientsPerMessage < globalMax {
		return dp.MaxRecipientsPerMessage
	}
	return globalMax
}
