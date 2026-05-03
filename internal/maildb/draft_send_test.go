package maildb

import (
	"testing"
)

func TestDraftOutboundAddressesDecodesAddressArray(t *testing.T) {
	t.Parallel()

	addresses, err := draftOutboundAddresses([]byte(`[{"name":"User","email":"user@example.net"}]`))
	if err != nil {
		t.Fatalf("draftOutboundAddresses returned error: %v", err)
	}
	if len(addresses) != 1 || addresses[0].Email != "user@example.net" || addresses[0].Name != "User" {
		t.Fatalf("addresses = %+v", addresses)
	}
}

func TestDraftOutboundAddressesAllowsEmptyJSON(t *testing.T) {
	t.Parallel()

	addresses, err := draftOutboundAddresses(nil)
	if err != nil {
		t.Fatalf("draftOutboundAddresses returned error: %v", err)
	}
	if len(addresses) != 0 {
		t.Fatalf("addresses = %+v", addresses)
	}
}
