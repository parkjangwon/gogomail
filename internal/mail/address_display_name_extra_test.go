package mail

import "testing"

func TestNormalizeAddressAcceptsDisplayName(t *testing.T) {
	got, err := NormalizeAddress(`Gogo Ops <Ops@Example.ORG>`)
	if err != nil {
		t.Fatalf("NormalizeAddress returned error: %v", err)
	}
	if got != "ops@example.org" {
		t.Fatalf("NormalizeAddress = %q, want ops@example.org", got)
	}
}
