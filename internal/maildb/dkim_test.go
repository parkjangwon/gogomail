package maildb

import "testing"

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
