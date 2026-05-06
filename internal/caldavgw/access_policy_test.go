package caldavgw

import (
	"context"
	"testing"

	"github.com/gogomail/gogomail/internal/accesspolicy"
	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/directory"
)

func TestDelegatedAccessPolicyAuthorizesCalendarDelegation(t *testing.T) {
	t.Parallel()

	checker := &fakeEffectiveDelegationChecker{allowed: true}
	recorder := &fakeDelegationAuditRepository{}
	policy := DelegatedAccessPolicy{
		Directory: fakeDirectoryResolver{
			"owner-1": {ID: "owner-1", Kind: directory.PrincipalKindUser, CompanyID: "company-1"},
			"actor-1": {ID: "actor-1", Kind: directory.PrincipalKindUser, CompanyID: "company-1"},
		},
		Authorizer: accesspolicy.DelegatedAccessAuthorizer{Checker: checker, AuditRepository: recorder},
	}
	decision, err := policy.AuthorizeCalendarAccess(context.Background(), AccessRequest{
		ActorUserID:  "actor-1",
		OwnerUserID:  "owner-1",
		RequiredRole: CalendarAccessRoleRead,
	})
	if err != nil {
		t.Fatalf("AuthorizeCalendarAccess returned error: %v", err)
	}
	if !decision.Allowed {
		t.Fatal("Allowed = false, want true")
	}
	if checker.last.Scope != directory.DelegationScopeCalendar || checker.last.RequiredRole != directory.DelegationRoleRead {
		t.Fatalf("delegation check = %+v", checker.last)
	}
	if len(recorder.logs) != 1 || recorder.logs[0].ActorID != "actor-1" || recorder.logs[0].TargetID != "owner-1" {
		t.Fatalf("audit logs = %+v", recorder.logs)
	}
}

func TestDelegatedAccessPolicyDeniesCrossCompanyPrincipals(t *testing.T) {
	t.Parallel()

	checker := &fakeEffectiveDelegationChecker{allowed: true}
	policy := DelegatedAccessPolicy{
		Directory: fakeDirectoryResolver{
			"owner-1": {ID: "owner-1", Kind: directory.PrincipalKindUser, CompanyID: "company-1"},
			"actor-1": {ID: "actor-1", Kind: directory.PrincipalKindUser, CompanyID: "company-2"},
		},
		Authorizer: accesspolicy.DelegatedAccessAuthorizer{Checker: checker, AuditRepository: &fakeDelegationAuditRepository{}},
	}
	decision, err := policy.AuthorizeCalendarAccess(context.Background(), AccessRequest{
		ActorUserID:  "actor-1",
		OwnerUserID:  "owner-1",
		RequiredRole: CalendarAccessRoleRead,
	})
	if err != nil {
		t.Fatalf("AuthorizeCalendarAccess returned error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("Allowed = true, want false")
	}
	if checker.last.CompanyID != "" {
		t.Fatalf("delegation checker was called: %+v", checker.last)
	}
}

type fakeDirectoryResolver map[string]directory.Principal

func (r fakeDirectoryResolver) ResolvePrincipal(_ context.Context, req directory.ResolvePrincipalRequest) (directory.Principal, error) {
	return r[req.ID], nil
}

type fakeEffectiveDelegationChecker struct {
	allowed bool
	last    directory.CheckDelegationRequest
}

func (c *fakeEffectiveDelegationChecker) CheckEffectiveDelegation(_ context.Context, req directory.CheckDelegationRequest) (bool, error) {
	c.last = req
	return c.allowed, nil
}

type fakeDelegationAuditRepository struct {
	logs []audit.Log
}

func (r *fakeDelegationAuditRepository) Insert(_ context.Context, log audit.Log) error {
	r.logs = append(r.logs, log)
	return nil
}
