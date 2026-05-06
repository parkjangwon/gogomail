package carddavgw

import (
	"context"
	"testing"

	"github.com/gogomail/gogomail/internal/accesspolicy"
	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/directory"
)

func TestDelegatedAccessPolicyAuthorizesContactsDelegation(t *testing.T) {
	t.Parallel()

	checker := &fakeEffectiveDelegationChecker{allowedRoles: map[string]bool{directory.DelegationRoleRead: true}}
	recorder := &fakeDelegationAuditRepository{}
	policy := DelegatedAccessPolicy{
		Directory: fakeDirectoryResolver{
			"owner-1": {ID: "owner-1", Kind: directory.PrincipalKindUser, CompanyID: "company-1"},
			"actor-1": {ID: "actor-1", Kind: directory.PrincipalKindUser, CompanyID: "company-1"},
		},
		Authorizer: accesspolicy.DelegatedAccessAuthorizer{Checker: checker, AuditRepository: recorder},
	}
	decision, err := policy.AuthorizeAddressBookAccess(context.Background(), AccessRequest{
		ActorUserID:  "actor-1",
		OwnerUserID:  "owner-1",
		RequiredRole: ContactsAccessRoleRead,
	})
	if err != nil {
		t.Fatalf("AuthorizeAddressBookAccess returned error: %v", err)
	}
	if !decision.Allowed {
		t.Fatal("Allowed = false, want true")
	}
	if len(decision.Privileges) != 1 || decision.Privileges[0] != PrivilegeRead {
		t.Fatalf("privileges = %+v, want read", decision.Privileges)
	}
	if len(checker.calls) != 4 ||
		checker.calls[0].Scope != directory.DelegationScopeContacts ||
		checker.calls[0].RequiredRole != directory.DelegationRoleRead ||
		checker.calls[1].RequiredRole != directory.DelegationRoleManage ||
		checker.calls[2].RequiredRole != directory.DelegationRoleWrite ||
		checker.calls[3].RequiredRole != directory.DelegationRoleRead {
		t.Fatalf("delegation checks = %+v", checker.calls)
	}
	if len(recorder.logs) != 1 || recorder.logs[0].ActorID != "actor-1" || recorder.logs[0].TargetID != "owner-1" {
		t.Fatalf("audit logs = %+v", recorder.logs)
	}
}

func TestDelegatedAccessPolicyDeniesCrossCompanyContactsPrincipals(t *testing.T) {
	t.Parallel()

	checker := &fakeEffectiveDelegationChecker{allowedRoles: map[string]bool{directory.DelegationRoleRead: true}}
	policy := DelegatedAccessPolicy{
		Directory: fakeDirectoryResolver{
			"owner-1": {ID: "owner-1", Kind: directory.PrincipalKindUser, CompanyID: "company-1"},
			"actor-1": {ID: "actor-1", Kind: directory.PrincipalKindUser, CompanyID: "company-2"},
		},
		Authorizer: accesspolicy.DelegatedAccessAuthorizer{Checker: checker, AuditRepository: &fakeDelegationAuditRepository{}},
	}
	decision, err := policy.AuthorizeAddressBookAccess(context.Background(), AccessRequest{
		ActorUserID:  "actor-1",
		OwnerUserID:  "owner-1",
		RequiredRole: ContactsAccessRoleRead,
	})
	if err != nil {
		t.Fatalf("AuthorizeAddressBookAccess returned error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("Allowed = true, want false")
	}
	if len(checker.calls) != 0 {
		t.Fatalf("delegation checker was called: %+v", checker.calls)
	}
}

func TestDelegatedAccessPolicyDeniesMissingContactsPrincipals(t *testing.T) {
	t.Parallel()

	checker := &fakeEffectiveDelegationChecker{allowedRoles: map[string]bool{directory.DelegationRoleRead: true}}
	policy := DelegatedAccessPolicy{
		Directory: fakeDirectoryResolver{
			"owner-1": {ID: "owner-1", Kind: directory.PrincipalKindUser, CompanyID: "company-1"},
		},
		Authorizer: accesspolicy.DelegatedAccessAuthorizer{Checker: checker, AuditRepository: &fakeDelegationAuditRepository{}},
	}
	decision, err := policy.AuthorizeAddressBookAccess(context.Background(), AccessRequest{
		ActorUserID:  "missing-actor",
		OwnerUserID:  "owner-1",
		RequiredRole: ContactsAccessRoleRead,
	})
	if err != nil {
		t.Fatalf("AuthorizeAddressBookAccess returned error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("Allowed = true, want false")
	}
	if len(checker.calls) != 0 {
		t.Fatalf("delegation checker was called: %+v", checker.calls)
	}
}

func TestDelegatedAccessPolicyReturnsHighestGrantedContactsPrivileges(t *testing.T) {
	t.Parallel()

	checker := &fakeEffectiveDelegationChecker{allowedRoles: map[string]bool{
		directory.DelegationRoleRead:  true,
		directory.DelegationRoleWrite: true,
	}}
	policy := DelegatedAccessPolicy{
		Directory: fakeDirectoryResolver{
			"owner-1": {ID: "owner-1", Kind: directory.PrincipalKindUser, CompanyID: "company-1"},
			"actor-1": {ID: "actor-1", Kind: directory.PrincipalKindUser, CompanyID: "company-1"},
		},
		Authorizer: accesspolicy.DelegatedAccessAuthorizer{Checker: checker, AuditRepository: &fakeDelegationAuditRepository{}},
	}
	decision, err := policy.AuthorizeAddressBookAccess(context.Background(), AccessRequest{
		ActorUserID:  "actor-1",
		OwnerUserID:  "owner-1",
		RequiredRole: ContactsAccessRoleRead,
	})
	if err != nil {
		t.Fatalf("AuthorizeAddressBookAccess returned error: %v", err)
	}
	want := []XMLName{PrivilegeRead, PrivilegeWriteContent, PrivilegeWriteProperties}
	if len(decision.Privileges) != len(want) {
		t.Fatalf("privileges = %+v, want %+v", decision.Privileges, want)
	}
	for i := range want {
		if decision.Privileges[i] != want[i] {
			t.Fatalf("privileges = %+v, want %+v", decision.Privileges, want)
		}
	}
}

type fakeDirectoryResolver map[string]directory.Principal

func (r fakeDirectoryResolver) ResolvePrincipal(_ context.Context, req directory.ResolvePrincipalRequest) (directory.Principal, error) {
	principal, ok := r[req.ID]
	if !ok {
		return directory.Principal{}, directory.ErrPrincipalNotFound
	}
	return principal, nil
}

type fakeEffectiveDelegationChecker struct {
	allowedRoles map[string]bool
	calls        []directory.CheckDelegationRequest
}

func (c *fakeEffectiveDelegationChecker) CheckEffectiveDelegation(_ context.Context, req directory.CheckDelegationRequest) (bool, error) {
	c.calls = append(c.calls, req)
	return c.allowedRoles[req.RequiredRole], nil
}

type fakeDelegationAuditRepository struct {
	logs []audit.Log
}

func (r *fakeDelegationAuditRepository) Insert(_ context.Context, log audit.Log) error {
	r.logs = append(r.logs, log)
	return nil
}
