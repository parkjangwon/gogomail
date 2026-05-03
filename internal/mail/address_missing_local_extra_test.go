package mail

import "testing"

func TestNormalizeAddressRejectsMissingLocalPart(t *testing.T) {
	if _, err := NormalizeAddress("@example.com"); err == nil {
		t.Fatal("NormalizeAddress accepted an address with no local part")
	}
}
