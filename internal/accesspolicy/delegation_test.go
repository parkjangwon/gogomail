package accesspolicy

import (
	"context"
	"errors"
	"testing"

	"github.com/gogomail/gogomail/internal/directory"
)

func TestDelegationEvaluatorAllowsEffectiveDelegation(t *testing.T) {
	t.Parallel()

	checker := &fakeDelegationChecker{allowed: true}
	decision, err := (DelegationEvaluator{Checker: checker}).CheckDelegatedAccess(context.Background(), DelegatedAccessRequest{
		CompanyID:    " company-1 ",
		Owner:        Principal(" RESOURCE ", " room-1 "),
		Actor:        Principal(" USER ", " user-1 "),
		Scope:        " Calendar ",
		RequiredRole: " WRITE ",
	})
	if err != nil {
		t.Fatalf("CheckDelegatedAccess returned error: %v", err)
	}
	if !decision.Allowed || decision.Reason != DecisionReasonDelegationAllowed {
		t.Fatalf("decision = %+v, want allowed", decision)
	}
	if decision.Scope != directory.DelegationScopeCalendar || decision.RequiredRole != directory.DelegationRoleWrite {
		t.Fatalf("decision scope/role = %+v", decision)
	}
	if checker.last.CompanyID != "company-1" ||
		checker.last.OwnerKind != directory.PrincipalKindResource ||
		checker.last.OwnerID != "room-1" ||
		checker.last.DelegateKind != directory.PrincipalKindUser ||
		checker.last.DelegateID != "user-1" ||
		checker.last.Scope != directory.DelegationScopeCalendar ||
		checker.last.RequiredRole != directory.DelegationRoleWrite ||
		!checker.last.ActiveOnly ||
		checker.last.MaxDepth != directory.DefaultMembershipMaxDepth {
		t.Fatalf("checker request = %+v", checker.last)
	}
}

func TestDelegationEvaluatorDeniesWithoutError(t *testing.T) {
	t.Parallel()

	decision, err := (DelegationEvaluator{Checker: &fakeDelegationChecker{}}).CheckDelegatedAccess(context.Background(), DelegatedAccessRequest{
		CompanyID:    "company-1",
		Owner:        Principal(directory.PrincipalKindResource, "room-1"),
		Actor:        Principal(directory.PrincipalKindUser, "user-1"),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: directory.DelegationRoleRead,
	})
	if err != nil {
		t.Fatalf("CheckDelegatedAccess returned error: %v", err)
	}
	if decision.Allowed || decision.Reason != DecisionReasonDelegationDenied {
		t.Fatalf("decision = %+v, want denied", decision)
	}
}

func TestDelegationEvaluatorRejectsMalformedRequestBeforeChecker(t *testing.T) {
	t.Parallel()

	checker := &fakeDelegationChecker{}
	_, err := (DelegationEvaluator{Checker: checker}).CheckDelegatedAccess(context.Background(), DelegatedAccessRequest{
		CompanyID:    "company-1",
		Owner:        Principal(directory.PrincipalKindResource, "room-1"),
		Actor:        Principal("calendar", "user-1"),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: directory.DelegationRoleRead,
	})
	if err == nil {
		t.Fatal("CheckDelegatedAccess accepted malformed actor principal kind")
	}
	if checker.called {
		t.Fatal("checker was called for malformed request")
	}
}

func TestDelegationEvaluatorPropagatesCheckerErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("database unavailable")
	_, err := (DelegationEvaluator{Checker: &fakeDelegationChecker{err: want}}).CheckDelegatedAccess(context.Background(), DelegatedAccessRequest{
		CompanyID:    "company-1",
		Owner:        Principal(directory.PrincipalKindResource, "room-1"),
		Actor:        Principal(directory.PrincipalKindUser, "user-1"),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: directory.DelegationRoleRead,
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestDelegationEvaluatorRequiresChecker(t *testing.T) {
	t.Parallel()

	_, err := (DelegationEvaluator{}).CheckDelegatedAccess(context.Background(), DelegatedAccessRequest{})
	if err == nil {
		t.Fatal("CheckDelegatedAccess accepted nil checker")
	}
}

func TestWebDAVPrivilegesForDecisionMapsRoles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		decision Decision
		want     []string
	}{
		{
			name:     "denied",
			decision: Decision{Allowed: false, RequiredRole: directory.DelegationRoleManage},
		},
		{
			name:     "read",
			decision: Decision{Allowed: true, RequiredRole: directory.DelegationRoleRead},
			want:     []string{WebDAVPrivilegeRead},
		},
		{
			name:     "write",
			decision: Decision{Allowed: true, RequiredRole: directory.DelegationRoleWrite},
			want:     []string{WebDAVPrivilegeRead, WebDAVPrivilegeWriteContent, WebDAVPrivilegeWriteProperties},
		},
		{
			name:     "manage",
			decision: Decision{Allowed: true, RequiredRole: directory.DelegationRoleManage},
			want: []string{
				WebDAVPrivilegeRead,
				WebDAVPrivilegeBind,
				WebDAVPrivilegeUnbind,
				WebDAVPrivilegeWriteContent,
				WebDAVPrivilegeWriteProperties,
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := WebDAVPrivilegesForDecision(tc.decision)
			if len(got) != len(tc.want) {
				t.Fatalf("privileges = %+v, want %+v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("privileges = %+v, want %+v", got, tc.want)
				}
			}
		})
	}
}

type fakeDelegationChecker struct {
	allowed bool
	err     error
	called  bool
	last    directory.CheckDelegationRequest
}

func (f *fakeDelegationChecker) CheckEffectiveDelegation(_ context.Context, req directory.CheckDelegationRequest) (bool, error) {
	f.called = true
	f.last = req
	if f.err != nil {
		return false, f.err
	}
	return f.allowed, nil
}
