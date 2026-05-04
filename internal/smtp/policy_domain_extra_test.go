package smtpd

import (
	"testing"
)

func TestEffectiveMaxBytesUsesGlobalWhenNoDomainPolicy(t *testing.T) {
	t.Parallel()

	if got := effectiveMaxBytes(10_000, nil); got != 10_000 {
		t.Fatalf("effectiveMaxBytes(10000, nil) = %d, want 10000", got)
	}
}

func TestEffectiveMaxBytesUsesGlobalWhenNotEnforce(t *testing.T) {
	t.Parallel()

	dp := &InboundDomainPolicy{InboundMode: "inherit", MaxMessageBytes: 100}
	if got := effectiveMaxBytes(10_000, dp); got != 10_000 {
		t.Fatalf("effectiveMaxBytes with inherit mode = %d, want 10000", got)
	}
}

func TestEffectiveMaxBytesUsesStricterDomainLimit(t *testing.T) {
	t.Parallel()

	dp := &InboundDomainPolicy{InboundMode: "enforce", MaxMessageBytes: 5_000}
	if got := effectiveMaxBytes(10_000, dp); got != 5_000 {
		t.Fatalf("effectiveMaxBytes with stricter domain = %d, want 5000", got)
	}
}

func TestEffectiveMaxBytesKeepsGlobalWhenDomainIsLarger(t *testing.T) {
	t.Parallel()

	dp := &InboundDomainPolicy{InboundMode: "enforce", MaxMessageBytes: 50_000}
	if got := effectiveMaxBytes(10_000, dp); got != 10_000 {
		t.Fatalf("effectiveMaxBytes with larger domain = %d, want 10000", got)
	}
}

func TestEffectiveMaxRecipientsUsesStricterDomainLimit(t *testing.T) {
	t.Parallel()

	dp := &InboundDomainPolicy{InboundMode: "enforce", MaxRecipientsPerMessage: 5}
	if got := effectiveMaxRecipients(100, dp); got != 5 {
		t.Fatalf("effectiveMaxRecipients with stricter domain = %d, want 5", got)
	}
}

func TestEffectiveMaxRecipientsKeepsGlobalWhenDomainIsLarger(t *testing.T) {
	t.Parallel()

	dp := &InboundDomainPolicy{InboundMode: "enforce", MaxRecipientsPerMessage: 200}
	if got := effectiveMaxRecipients(100, dp); got != 100 {
		t.Fatalf("effectiveMaxRecipients with larger domain = %d, want 100", got)
	}
}
