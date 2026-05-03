package mail

import "testing"

func TestNormalizeAddressRejectsMissingDomain(t *testing.T) {
	if _, err := NormalizeAddress("localpart@"); err == nil {
		t.Fatal("NormalizeAddress accepted an address with no domain")
	}
}
