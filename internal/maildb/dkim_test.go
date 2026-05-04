package maildb

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateDKIMKeyListRequestRejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	if err := ValidateDKIMKeyListRequest(DKIMKeyListRequest{Status: "revoked"}); err == nil {
		t.Fatal("ValidateDKIMKeyListRequest accepted unknown status")
	}
}

func TestValidateDKIMKeyListRequestAcceptsKnownStatuses(t *testing.T) {
	t.Parallel()

	for _, status := range []string{"", "active", "inactive", " Active "} {
		status := status
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			if err := ValidateDKIMKeyListRequest(DKIMKeyListRequest{Status: status}); err != nil {
				t.Fatalf("ValidateDKIMKeyListRequest(%q) returned error: %v", status, err)
			}
		})
	}
}

func TestDKIMKeyAuditDetailDoesNotIncludePrivateKeyMaterial(t *testing.T) {
	t.Parallel()

	detail, err := dkimKeyAuditDetail(dkimKeyAuditView{
		ID:                     "key-1",
		DomainID:               "domain-1",
		Selector:               "s1",
		Status:                 "active",
		PublicKeyDNSConfigured: true,
		DNSCheckID:             "check-1",
		DNSStatus:              "ok",
		DNSVerified:            true,
	})
	if err != nil {
		t.Fatalf("dkimKeyAuditDetail returned error: %v", err)
	}
	if strings.Contains(string(detail), "PRIVATE KEY") || strings.Contains(string(detail), "private_key") {
		t.Fatalf("audit detail leaked private key material: %s", detail)
	}
	var body struct {
		ID                     string `json:"dkim_key_id"`
		DomainID               string `json:"domain_id"`
		Selector               string `json:"selector"`
		Status                 string `json:"status"`
		PublicKeyDNSConfigured bool   `json:"public_key_dns_configured"`
		DNSCheckID             string `json:"dns_check_id"`
		DNSStatus              string `json:"dns_status"`
		DNSVerified            bool   `json:"dns_verified"`
	}
	if err := json.Unmarshal(detail, &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body.ID != "key-1" ||
		body.DomainID != "domain-1" ||
		body.Selector != "s1" ||
		body.Status != "active" ||
		!body.PublicKeyDNSConfigured ||
		body.DNSCheckID != "check-1" ||
		body.DNSStatus != "ok" ||
		!body.DNSVerified {
		t.Fatalf("detail = %+v", body)
	}
}
