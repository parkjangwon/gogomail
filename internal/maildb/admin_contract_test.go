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

func TestValidateCreateDomainRequestRejectsInvalidName(t *testing.T) {
	t.Parallel()

	err := ValidateCreateDomainRequest(CreateDomainRequest{
		CompanyID: "company-1",
		Name:      "bad domain/example.com",
	})
	if err == nil {
		t.Fatal("ValidateCreateDomainRequest accepted invalid domain name")
	}
}

func TestValidateUpdateUserStatusRequestRejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	if err := ValidateUpdateUserStatusRequest(UpdateUserStatusRequest{ID: "user-1", Status: "paused"}); err == nil {
		t.Fatal("ValidateUpdateUserStatusRequest accepted unknown status")
	}
}

func TestValidateCreateUserRequestRejectsInvalidUsername(t *testing.T) {
	t.Parallel()

	err := ValidateCreateUserRequest(CreateUserRequest{
		DomainID:    "domain-1",
		Username:    "admin@example.com",
		DisplayName: "Admin",
		Address:     "admin@example.com",
	})
	if err == nil {
		t.Fatal("ValidateCreateUserRequest accepted invalid username")
	}
}

func TestValidateCreateUserRequestRejectsInvalidAddress(t *testing.T) {
	t.Parallel()

	err := ValidateCreateUserRequest(CreateUserRequest{
		DomainID:    "domain-1",
		Username:    "admin",
		DisplayName: "Admin",
		Address:     "not an address",
	})
	if err == nil {
		t.Fatal("ValidateCreateUserRequest accepted invalid address")
	}
}

func TestNormalizeAdminStatusTrimsAndLowers(t *testing.T) {
	t.Parallel()

	if got := normalizeAdminStatus(" Suspended "); got != "suspended" {
		t.Fatalf("status = %q, want suspended", got)
	}
}
