package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/auditretention"
)

type fakeAuditRetentionStore struct {
	policy      *auditretention.Policy
	setCalls    int
	deleteCalls int
	lastSetDays int
	runs        []auditretention.RunRecord
	lastList    auditretention.RunListRequest
	err         error
}

func (f *fakeAuditRetentionStore) GetPolicy(_ context.Context, companyID string) (*auditretention.Policy, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.policy, nil
}

func (f *fakeAuditRetentionStore) SetPolicy(_ context.Context, companyID string, retentionDays int, updatedBy string) (*auditretention.Policy, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.setCalls++
	f.lastSetDays = retentionDays
	p := &auditretention.Policy{CompanyID: companyID, RetentionDays: retentionDays, UpdatedBy: updatedBy, UpdatedAt: time.Now().UTC()}
	f.policy = p
	return p, nil
}

func (f *fakeAuditRetentionStore) DeletePolicy(_ context.Context, companyID string) error {
	if f.err != nil {
		return f.err
	}
	f.deleteCalls++
	f.policy = nil
	return nil
}

func (f *fakeAuditRetentionStore) ListRuns(_ context.Context, req auditretention.RunListRequest) ([]auditretention.RunRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.lastList = req
	return f.runs, nil
}

func TestAdminServiceSetAuditRetentionPolicyPersistsAndAudits(t *testing.T) {
	store := &fakeAuditRetentionStore{}
	writer := &fakeAuditWriter{}
	service := adminService{auditRetention: store, audit: writer}

	policy, err := service.SetAuditRetentionPolicy(t.Context(), "company-1", 90)
	if err != nil {
		t.Fatalf("SetAuditRetentionPolicy: %v", err)
	}
	if policy == nil || policy.RetentionDays != 90 {
		t.Fatalf("returned policy = %+v", policy)
	}
	if store.setCalls != 1 || store.lastSetDays != 90 {
		t.Fatalf("SetPolicy calls = %d, lastDays = %d", store.setCalls, store.lastSetDays)
	}
	if writer.insertCalls != 1 {
		t.Fatalf("audit insert calls = %d, want 1", writer.insertCalls)
	}
	if writer.log.Action != "audit_retention.policy_set" || writer.log.CompanyID != "company-1" {
		t.Fatalf("audit log = %+v", writer.log)
	}
}

func TestAdminServiceSetAuditRetentionPolicyZeroDeletes(t *testing.T) {
	store := &fakeAuditRetentionStore{policy: &auditretention.Policy{CompanyID: "company-1", RetentionDays: 90}}
	writer := &fakeAuditWriter{}
	service := adminService{auditRetention: store, audit: writer}

	policy, err := service.SetAuditRetentionPolicy(t.Context(), "company-1", 0)
	if err != nil {
		t.Fatalf("SetAuditRetentionPolicy(0): %v", err)
	}
	if policy != nil {
		t.Fatalf("keep-forever policy = %+v, want nil", policy)
	}
	if store.deleteCalls != 1 || store.setCalls != 0 {
		t.Fatalf("delete calls = %d, set calls = %d", store.deleteCalls, store.setCalls)
	}
	if writer.insertCalls != 1 || writer.log.Action != "audit_retention.policy_cleared" {
		t.Fatalf("audit log = %+v (calls %d)", writer.log, writer.insertCalls)
	}
}

func TestAdminServiceSetAuditRetentionPolicyRejectsInvalidDays(t *testing.T) {
	store := &fakeAuditRetentionStore{}
	service := adminService{auditRetention: store, audit: &fakeAuditWriter{}}
	if _, err := service.SetAuditRetentionPolicy(t.Context(), "company-1", 45); err == nil {
		t.Fatal("SetAuditRetentionPolicy(45): want validation error")
	}
	if store.setCalls != 0 || store.deleteCalls != 0 {
		t.Fatalf("no store mutation expected on invalid input: set=%d delete=%d", store.setCalls, store.deleteCalls)
	}
}

func TestAdminServiceGetAuditRetentionPolicyKeepForever(t *testing.T) {
	store := &fakeAuditRetentionStore{policy: nil}
	service := adminService{auditRetention: store}
	policy, err := service.GetAuditRetentionPolicy(t.Context(), "company-1")
	if err != nil {
		t.Fatalf("GetAuditRetentionPolicy: %v", err)
	}
	if policy != nil {
		t.Fatalf("policy = %+v, want nil (keep forever)", policy)
	}
}

func TestAdminServiceListAuditRetentionRunsDelegates(t *testing.T) {
	store := &fakeAuditRetentionStore{runs: []auditretention.RunRecord{{ID: "audit-retention-1", CompanyID: "company-1"}}}
	service := adminService{auditRetention: store}
	runs, err := service.ListAuditRetentionRuns(t.Context(), auditretention.RunListRequest{CompanyID: "company-1", Limit: 5})
	if err != nil {
		t.Fatalf("ListAuditRetentionRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != "audit-retention-1" {
		t.Fatalf("runs = %+v", runs)
	}
	if store.lastList.CompanyID != "company-1" || store.lastList.Limit != 5 {
		t.Fatalf("last list request = %+v", store.lastList)
	}
}

func TestAdminServiceAuditRetentionUnconfigured(t *testing.T) {
	service := adminService{}
	if _, err := service.GetAuditRetentionPolicy(context.Background(), "c1"); err == nil {
		t.Error("GetAuditRetentionPolicy unconfigured: want error")
	}
	if _, err := service.SetAuditRetentionPolicy(context.Background(), "c1", 30); err == nil {
		t.Error("SetAuditRetentionPolicy unconfigured: want error")
	}
	if _, err := service.ListAuditRetentionRuns(context.Background(), auditretention.RunListRequest{CompanyID: "c1"}); err == nil {
		t.Error("ListAuditRetentionRuns unconfigured: want error")
	}
}

// TestAdminServiceSetAuditRetentionPolicyAuditFailurePropagates ensures a
// failed audit write is surfaced (the change should not silently proceed).
func TestAdminServiceSetAuditRetentionPolicyAuditFailurePropagates(t *testing.T) {
	store := &fakeAuditRetentionStore{}
	writer := &fakeAuditWriter{err: fmt.Errorf("audit down")}
	service := adminService{auditRetention: store, audit: writer}
	if _, err := service.SetAuditRetentionPolicy(t.Context(), "company-1", 30); err == nil {
		t.Fatal("SetAuditRetentionPolicy: want error when audit write fails")
	}
}
