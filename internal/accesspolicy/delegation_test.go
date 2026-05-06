package accesspolicy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gogomail/gogomail/internal/audit"
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

func TestDelegationAuditRecorderRecordsDelegatedAccess(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepository{}
	err := (DelegationAuditRecorder{Repository: repo}).RecordDelegatedAccess(context.Background(), DelegatedAccessRequest{
		CompanyID:    "company-1",
		Owner:        Principal(directory.PrincipalKindResource, "room-1"),
		Actor:        Principal(directory.PrincipalKindUser, "user-1"),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: directory.DelegationRoleRead,
	}, Decision{Allowed: false})
	if err != nil {
		t.Fatalf("RecordDelegatedAccess returned error: %v", err)
	}
	if !repo.called ||
		repo.log.CompanyID != "company-1" ||
		repo.log.ActorID != "user-1" ||
		repo.log.TargetType != directory.PrincipalKindResource ||
		repo.log.TargetID != "room-1" ||
		repo.log.Result != AuditResultDelegationDenied {
		t.Fatalf("recorded audit log = %+v called=%v", repo.log, repo.called)
	}
}

func TestDelegationAuditRecorderRequiresRepository(t *testing.T) {
	t.Parallel()

	err := (DelegationAuditRecorder{}).RecordDelegatedAccess(context.Background(), DelegatedAccessRequest{}, Decision{})
	if err == nil {
		t.Fatal("RecordDelegatedAccess accepted nil repository")
	}
}

func TestDelegationAuditRecorderPropagatesInsertError(t *testing.T) {
	t.Parallel()

	want := errors.New("audit database unavailable")
	err := (DelegationAuditRecorder{Repository: &fakeAuditRepository{err: want}}).RecordDelegatedAccess(context.Background(), DelegatedAccessRequest{
		CompanyID:    "company-1",
		Owner:        Principal(directory.PrincipalKindResource, "room-1"),
		Actor:        Principal(directory.PrincipalKindUser, "user-1"),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: directory.DelegationRoleRead,
	}, Decision{Allowed: true})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestDelegatedAccessAuthorizerRecordsAllowedDecision(t *testing.T) {
	t.Parallel()

	checker := &fakeDelegationChecker{allowed: true}
	repo := &fakeAuditRepository{}
	decision, err := (DelegatedAccessAuthorizer{Checker: checker, AuditRepository: repo}).CheckAndRecordDelegatedAccess(context.Background(), DelegatedAccessRequest{
		CompanyID:    " company-1 ",
		Owner:        Principal(" RESOURCE ", " room-1 "),
		Actor:        Principal(" USER ", " user-1 "),
		Scope:        " Calendar ",
		RequiredRole: " WRITE ",
	})
	if err != nil {
		t.Fatalf("CheckAndRecordDelegatedAccess returned error: %v", err)
	}
	if !decision.Allowed ||
		decision.Reason != DecisionReasonDelegationAllowed ||
		decision.Scope != directory.DelegationScopeCalendar ||
		decision.RequiredRole != directory.DelegationRoleWrite {
		t.Fatalf("decision = %+v, want normalized allowed decision", decision)
	}
	if !checker.called || checker.last.CompanyID != "company-1" || checker.last.OwnerKind != directory.PrincipalKindResource || checker.last.RequiredRole != directory.DelegationRoleWrite {
		t.Fatalf("checker request = %+v called=%v", checker.last, checker.called)
	}
	if !repo.called ||
		repo.log.CompanyID != "company-1" ||
		repo.log.ActorID != "user-1" ||
		repo.log.TargetType != directory.PrincipalKindResource ||
		repo.log.TargetID != "room-1" ||
		repo.log.Result != AuditResultDelegationAllowed {
		t.Fatalf("recorded audit log = %+v called=%v", repo.log, repo.called)
	}
}

func TestDelegatedAccessAuthorizerRecordsDeniedDecision(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditRepository{}
	decision, err := (DelegatedAccessAuthorizer{Checker: &fakeDelegationChecker{}, AuditRepository: repo}).CheckAndRecordDelegatedAccess(context.Background(), DelegatedAccessRequest{
		CompanyID:    "company-1",
		Owner:        Principal(directory.PrincipalKindResource, "room-1"),
		Actor:        Principal(directory.PrincipalKindUser, "user-1"),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: directory.DelegationRoleManage,
	})
	if err != nil {
		t.Fatalf("CheckAndRecordDelegatedAccess returned error: %v", err)
	}
	if decision.Allowed || decision.Reason != DecisionReasonDelegationDenied {
		t.Fatalf("decision = %+v, want denied decision", decision)
	}
	if !repo.called || repo.log.Result != AuditResultDelegationDenied {
		t.Fatalf("recorded audit log = %+v called=%v", repo.log, repo.called)
	}
}

func TestDelegatedAccessAuthorizerSkipsAuditOnCheckerError(t *testing.T) {
	t.Parallel()

	want := errors.New("directory unavailable")
	repo := &fakeAuditRepository{}
	_, err := (DelegatedAccessAuthorizer{Checker: &fakeDelegationChecker{err: want}, AuditRepository: repo}).CheckAndRecordDelegatedAccess(context.Background(), DelegatedAccessRequest{
		CompanyID:    "company-1",
		Owner:        Principal(directory.PrincipalKindResource, "room-1"),
		Actor:        Principal(directory.PrincipalKindUser, "user-1"),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: directory.DelegationRoleRead,
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if repo.called {
		t.Fatalf("audit repository called after checker error: %+v", repo.log)
	}
}

func TestDelegatedAccessAuthorizerReturnsAuditInsertError(t *testing.T) {
	t.Parallel()

	want := errors.New("audit database unavailable")
	decision, err := (DelegatedAccessAuthorizer{Checker: &fakeDelegationChecker{allowed: true}, AuditRepository: &fakeAuditRepository{err: want}}).CheckAndRecordDelegatedAccess(context.Background(), DelegatedAccessRequest{
		CompanyID:    "company-1",
		Owner:        Principal(directory.PrincipalKindResource, "room-1"),
		Actor:        Principal(directory.PrincipalKindUser, "user-1"),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: directory.DelegationRoleRead,
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if decision.Allowed {
		t.Fatalf("decision = %+v, want zero decision on audit failure", decision)
	}
}

func TestDelegatedAccessAuthorizerRequiresDependencies(t *testing.T) {
	t.Parallel()

	req := DelegatedAccessRequest{
		CompanyID:    "company-1",
		Owner:        Principal(directory.PrincipalKindResource, "room-1"),
		Actor:        Principal(directory.PrincipalKindUser, "user-1"),
		Scope:        directory.DelegationScopeCalendar,
		RequiredRole: directory.DelegationRoleRead,
	}
	if _, err := (DelegatedAccessAuthorizer{AuditRepository: &fakeAuditRepository{}}).CheckAndRecordDelegatedAccess(context.Background(), req); err == nil {
		t.Fatal("CheckAndRecordDelegatedAccess accepted nil checker")
	}
	if _, err := (DelegatedAccessAuthorizer{Checker: &fakeDelegationChecker{}}).CheckAndRecordDelegatedAccess(context.Background(), req); err == nil {
		t.Fatal("CheckAndRecordDelegatedAccess accepted nil audit repository")
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

type fakeAuditRepository struct {
	err    error
	called bool
	log    audit.Log
}

func (f *fakeAuditRepository) Insert(_ context.Context, log audit.Log) error {
	f.called = true
	f.log = log
	return f.err
}
