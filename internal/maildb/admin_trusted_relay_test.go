package maildb

import (
	"encoding/json"
	"testing"
)

func TestValidateCreateTrustedRelayRequestAcceptsCIDRAndPlainIP(t *testing.T) {
	t.Parallel()

	for _, cidr := range []string{"192.0.2.0/24", "2001:db8::/32", "192.0.2.1", "2001:db8::1"} {
		if err := ValidateCreateTrustedRelayRequest(CreateTrustedRelayRequest{CIDR: cidr}); err != nil {
			t.Fatalf("ValidateCreateTrustedRelayRequest(%q) returned error: %v", cidr, err)
		}
	}
}

func TestNormalizeTrustedRelayCIDRCanonicalizesPlainIP(t *testing.T) {
	t.Parallel()

	if got, err := normalizeTrustedRelayCIDR("192.0.2.1"); err != nil || got != "192.0.2.1/32" {
		t.Fatalf("normalizeTrustedRelayCIDR IPv4 = %q, %v", got, err)
	}
	if got, err := normalizeTrustedRelayCIDR("2001:db8::1"); err != nil || got != "2001:db8::1/128" {
		t.Fatalf("normalizeTrustedRelayCIDR IPv6 = %q, %v", got, err)
	}
}

func TestValidateCreateTrustedRelayRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	for _, req := range []CreateTrustedRelayRequest{
		{},
		{CIDR: "not an ip"},
		{CIDR: "192.0.2.0/33"},
		{CIDR: "192.0.2.0/24", Description: "bad\nline"},
	} {
		if err := ValidateCreateTrustedRelayRequest(req); err == nil {
			t.Fatalf("ValidateCreateTrustedRelayRequest(%+v) returned nil", req)
		}
	}
}

func TestTrustedRelayAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := trustedRelayAuditDetail(TrustedRelayView{
		ID:          "relay-1",
		CIDR:        "192.0.2.0/24",
		Description: "edge relay",
	})
	if err != nil {
		t.Fatalf("trustedRelayAuditDetail returned error: %v", err)
	}
	var body map[string]string
	if err := json.Unmarshal(detail, &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body["trusted_relay_id"] != "relay-1" || body["cidr"] != "192.0.2.0/24" || body["description"] != "edge relay" {
		t.Fatalf("detail = %+v", body)
	}
}
