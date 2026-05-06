package accesspolicy

import (
	"context"
	"encoding/json"
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

func TestDelegatedAccessAuditDetailNormalizesDecision(t *testing.T) {
	t.Parallel()

	detail, err := DelegatedAccessAuditDetail(DelegatedAccessRequest{
		CompanyID:    " company-1 ",
		Owner:        Principal(" RESOURCE ", " room-1 "),
		Actor:        Principal(" USER ", " user-1 "),
		Scope:        " Calendar ",
		RequiredRole: " WRITE ",
	}, Decision{Allowed: true, Reason: DecisionReasonDelegationAllowed})
	if err != nil {
		t.Fatalf("DelegatedAccessAuditDetail returned error: %v", err)
	}
	var got struct {
		CompanyID        string   `json:"company_id"`
		OwnerKind        string   `json:"owner_kind"`
		OwnerID          string   `json:"owner_id"`
		ActorKind        string   `json:"actor_kind"`
		ActorID          string   `json:"actor_id"`
		Scope            string   `json:"scope"`
		RequiredRole     string   `json:"required_role"`
		Allowed          bool     `json:"allowed"`
		Reason           string   `json:"reason"`
		WebDAVPrivileges []string `json:"webdav_privileges"`
	}
	if err := json.Unmarshal(detail, &got); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if got.CompanyID != "company-1" ||
		got.OwnerKind != directory.PrincipalKindResource ||
		got.OwnerID != "room-1" ||
		got.ActorKind != directory.PrincipalKindUser ||
		got.ActorID != "user-1" ||
		got.Scope != directory.DelegationScopeCalendar ||
		got.RequiredRole != directory.DelegationRoleWrite ||
		!got.Allowed ||
		got.Reason != DecisionReasonDelegationAllowed {
		t.Fatalf("audit detail = %+v", got)
	}
	wantPrivileges := []string{WebDAVPrivilegeRead, WebDAVPrivilegeWriteContent, WebDAVPrivilegeWriteProperties}
	if len(got.WebDAVPrivileges) != len(wantPrivileges) {
		t.Fatalf("webdav privileges = %+v, want %+v", got.WebDAVPrivileges, wantPrivileges)
	}
	for i := range wantPrivileges {
		if got.WebDAVPrivileges[i] != wantPrivileges[i] {
			t.Fatalf("webdav privileges = %+v, want %+v", got.WebDAVPrivileges, wantPrivileges)
		}
	}
}

func TestDelegatedAccessAuditDetailOmitsDeniedPrivileges(t *testing.T) {
	t.Parallel()

	detail, err := DelegatedAccessAuditDetail(DelegatedAccessRequest{
		CompanyID:    "company-1",
		Owner:        Principal(directory.PrincipalKindResource, "room-1"),
		Actor:        Principal(directory.PrincipalKindUser, "user-1"),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: directory.DelegationRoleManage,
	}, Decision{Allowed: false})
	if err != nil {
		t.Fatalf("DelegatedAccessAuditDetail returned error: %v", err)
	}
	var got struct {
		Allowed          bool     `json:"allowed"`
		Reason           string   `json:"reason"`
		WebDAVPrivileges []string `json:"webdav_privileges"`
	}
	if err := json.Unmarshal(detail, &got); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if got.Allowed || got.Reason != DecisionReasonDelegationDenied || len(got.WebDAVPrivileges) != 0 {
		t.Fatalf("audit detail = %+v, want denied without privileges", got)
	}
}

func TestDelegatedAccessAuditDetailNormalizesUnsupportedReason(t *testing.T) {
	t.Parallel()

	req := DelegatedAccessRequest{
		CompanyID:    "company-1",
		Owner:        Principal(directory.PrincipalKindResource, "room-1"),
		Actor:        Principal(directory.PrincipalKindUser, "user-1"),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: directory.DelegationRoleRead,
	}
	detail, err := DelegatedAccessAuditDetail(req, Decision{Allowed: true, Reason: "user supplied\nreason"})
	if err != nil {
		t.Fatalf("DelegatedAccessAuditDetail returned error: %v", err)
	}
	var got struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(detail, &got); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if got.Reason != DecisionReasonDelegationAllowed {
		t.Fatalf("reason = %q, want normalized allowed reason", got.Reason)
	}

	detail, err = DelegatedAccessAuditDetail(req, Decision{Allowed: false, Reason: DecisionReasonDelegationAllowed})
	if err != nil {
		t.Fatalf("DelegatedAccessAuditDetail returned error: %v", err)
	}
	if err := json.Unmarshal(detail, &got); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if got.Reason != DecisionReasonDelegationDenied {
		t.Fatalf("reason = %q, want normalized denied reason", got.Reason)
	}
}

func TestDelegatedAccessAuditDetailRejectsMalformedRequest(t *testing.T) {
	t.Parallel()

	_, err := DelegatedAccessAuditDetail(DelegatedAccessRequest{
		CompanyID:    "company-1",
		Owner:        Principal(directory.PrincipalKindResource, "room-1"),
		Actor:        Principal("calendar", "user-1"),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: directory.DelegationRoleRead,
	}, Decision{Allowed: true})
	if err == nil {
		t.Fatal("DelegatedAccessAuditDetail accepted malformed actor principal kind")
	}
}

func TestDelegatedAccessAuditLogBuildsStableEnvelope(t *testing.T) {
	t.Parallel()

	log, err := DelegatedAccessAuditLog(DelegatedAccessRequest{
		CompanyID:    " company-1 ",
		Owner:        Principal(" RESOURCE ", " room-1 "),
		Actor:        Principal(" USER ", " user-1 "),
		Scope:        " Calendar ",
		RequiredRole: " MANAGE ",
	}, Decision{Allowed: true, Reason: "caller-specific text"})
	if err != nil {
		t.Fatalf("DelegatedAccessAuditLog returned error: %v", err)
	}
	if log.CompanyID != "company-1" ||
		log.ActorID != "user-1" ||
		log.Category != AuditCategoryAccess ||
		log.Action != AuditActionDelegatedAccess ||
		log.TargetType != directory.PrincipalKindResource ||
		log.TargetID != "room-1" ||
		log.Result != AuditResultDelegationAllowed {
		t.Fatalf("audit log = %+v", log)
	}
	var detail struct {
		Reason           string   `json:"reason"`
		WebDAVPrivileges []string `json:"webdav_privileges"`
	}
	if err := json.Unmarshal(log.Detail, &detail); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if detail.Reason != DecisionReasonDelegationAllowed {
		t.Fatalf("reason = %q, want normalized allowed reason", detail.Reason)
	}
	wantPrivileges := []string{
		WebDAVPrivilegeRead,
		WebDAVPrivilegeBind,
		WebDAVPrivilegeUnbind,
		WebDAVPrivilegeWriteContent,
		WebDAVPrivilegeWriteProperties,
	}
	if len(detail.WebDAVPrivileges) != len(wantPrivileges) {
		t.Fatalf("webdav privileges = %+v, want %+v", detail.WebDAVPrivileges, wantPrivileges)
	}
	for i := range wantPrivileges {
		if detail.WebDAVPrivileges[i] != wantPrivileges[i] {
			t.Fatalf("webdav privileges = %+v, want %+v", detail.WebDAVPrivileges, wantPrivileges)
		}
	}
}

func TestDelegatedAccessAuditLogRejectsMalformedRequest(t *testing.T) {
	t.Parallel()

	_, err := DelegatedAccessAuditLog(DelegatedAccessRequest{
		CompanyID:    "company-1",
		Owner:        Principal(directory.PrincipalKindResource, "room-1"),
		Actor:        Principal("calendar", "user-1"),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: directory.DelegationRoleRead,
	}, Decision{Allowed: false})
	if err == nil {
		t.Fatal("DelegatedAccessAuditLog accepted malformed actor principal kind")
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
