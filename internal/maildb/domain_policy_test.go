package maildb

import (
	"testing"
	"time"
)

func TestDomainPolicyFromJSONDefaultsToInherit(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	policy, err := domainPolicyFromJSON("domain-1", nil, updatedAt)
	if err != nil {
		t.Fatalf("domainPolicyFromJSON returned error: %v", err)
	}
	if policy.DomainID != "domain-1" || policy.InboundMode != "inherit" || policy.OutboundMode != "inherit" {
		t.Fatalf("policy = %+v", policy)
	}
	if !policy.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("UpdatedAt = %s, want %s", policy.UpdatedAt, updatedAt)
	}
}

func TestDomainPolicyFromJSONNormalizesBlankModes(t *testing.T) {
	t.Parallel()

	policy, err := domainPolicyFromJSON("domain-1", []byte(`{"inbound_mode":"","outbound_mode":"","max_recipients_per_message":5}`), time.Time{})
	if err != nil {
		t.Fatalf("domainPolicyFromJSON returned error: %v", err)
	}
	if policy.InboundMode != "inherit" || policy.OutboundMode != "inherit" || policy.MaxRecipientsPerMessage != 5 {
		t.Fatalf("policy = %+v", policy)
	}
}

func TestDomainPolicyFromJSONRejectsInvalidStoredValues(t *testing.T) {
	t.Parallel()

	for _, raw := range [][]byte{
		[]byte(`{"outbound_mode":"panic"}`),
		[]byte(`{"max_message_bytes":-1}`),
		[]byte(`{"max_recipients_per_message":-1}`),
	} {
		if _, err := domainPolicyFromJSON("domain-1", raw, time.Time{}); err == nil {
			t.Fatalf("domainPolicyFromJSON(%s) returned nil", raw)
		}
	}
}
