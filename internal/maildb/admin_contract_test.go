package maildb

import "testing"

func TestValidateUpdateDomainStatusRequestRejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	if err := ValidateUpdateDomainStatusRequest(UpdateDomainStatusRequest{ID: "domain-1", Status: "paused"}); err == nil {
		t.Fatal("ValidateUpdateDomainStatusRequest accepted unknown status")
	}
}

func TestValidateUpdateDomainStatusRequestAcceptsSuspended(t *testing.T) {
	t.Parallel()

	if err := ValidateUpdateDomainStatusRequest(UpdateDomainStatusRequest{ID: "domain-1", Status: "suspended"}); err != nil {
		t.Fatalf("ValidateUpdateDomainStatusRequest returned error: %v", err)
	}
}

func TestValidateUpdateUserStatusRequestRejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	if err := ValidateUpdateUserStatusRequest(UpdateUserStatusRequest{ID: "user-1", Status: "paused"}); err == nil {
		t.Fatal("ValidateUpdateUserStatusRequest accepted unknown status")
	}
}

func TestNormalizeAdminStatusTrimsAndLowers(t *testing.T) {
	t.Parallel()

	if got := normalizeAdminStatus(" Suspended "); got != "suspended" {
		t.Fatalf("status = %q, want suspended", got)
	}
}
