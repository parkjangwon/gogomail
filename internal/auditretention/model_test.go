package auditretention

import (
	"strings"
	"testing"
	"time"
)

func TestValidateRetentionDays(t *testing.T) {
	t.Parallel()

	valid := []int{0, 30, 90, 180, 365, 730}
	for _, days := range valid {
		if err := ValidateRetentionDays(days); err != nil {
			t.Errorf("ValidateRetentionDays(%d) = %v, want nil", days, err)
		}
	}

	invalid := []int{-1, 1, 29, 31, 60, 100, 366, 731, 1000}
	for _, days := range invalid {
		if err := ValidateRetentionDays(days); err == nil {
			t.Errorf("ValidateRetentionDays(%d) = nil, want error", days)
		}
	}
}

func TestNormalizeRunListRequest(t *testing.T) {
	t.Parallel()

	if _, err := normalizeRunListRequest(RunListRequest{}); err == nil {
		t.Fatal("normalizeRunListRequest with empty company id: want error")
	}
	if _, err := normalizeRunListRequest(RunListRequest{CompanyID: "c1", Limit: -1}); err == nil {
		t.Fatal("normalizeRunListRequest negative limit: want error")
	}
	req, err := normalizeRunListRequest(RunListRequest{CompanyID: " c1 "})
	if err != nil {
		t.Fatalf("normalizeRunListRequest: %v", err)
	}
	if req.CompanyID != "c1" {
		t.Fatalf("company id = %q, want trimmed c1", req.CompanyID)
	}
	if req.Limit != MaxRunListLimit {
		t.Fatalf("default limit = %d, want %d", req.Limit, MaxRunListLimit)
	}
	over, err := normalizeRunListRequest(RunListRequest{CompanyID: "c1", Limit: MaxRunListLimit + 50})
	if err != nil {
		t.Fatalf("normalizeRunListRequest over limit: %v", err)
	}
	if over.Limit != MaxRunListLimit {
		t.Fatalf("clamped limit = %d, want %d", over.Limit, MaxRunListLimit)
	}
	if _, err := normalizeRunListRequest(RunListRequest{CompanyID: "c1", Status: "bogus"}); err == nil {
		t.Fatal("normalizeRunListRequest bad status: want error")
	}
}

func TestNormalizeRunRecord(t *testing.T) {
	t.Parallel()

	now := func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	cutoff := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)

	// Missing company id.
	if _, err := normalizeRunRecord(RunRecord{RetentionDays: 30, Cutoff: cutoff}, now); err == nil {
		t.Fatal("normalizeRunRecord missing company id: want error")
	}
	// Non-positive retention days.
	if _, err := normalizeRunRecord(RunRecord{CompanyID: "c1", Cutoff: cutoff}, now); err == nil {
		t.Fatal("normalizeRunRecord zero retention days: want error")
	}
	// Missing cutoff.
	if _, err := normalizeRunRecord(RunRecord{CompanyID: "c1", RetentionDays: 30}, now); err == nil {
		t.Fatal("normalizeRunRecord missing cutoff: want error")
	}

	rec, err := normalizeRunRecord(RunRecord{
		CompanyID:     "c1",
		RetentionDays: 90,
		Cutoff:        cutoff,
		RowsDeleted:   5,
	}, now)
	if err != nil {
		t.Fatalf("normalizeRunRecord: %v", err)
	}
	if !strings.HasPrefix(rec.ID, "audit-retention-") {
		t.Fatalf("generated id = %q, want audit-retention- prefix", rec.ID)
	}
	if rec.Status != RunStatusCompleted {
		t.Fatalf("default status = %q, want completed", rec.Status)
	}
	if rec.StartedAt.IsZero() || rec.FinishedAt.IsZero() {
		t.Fatal("timestamps must be populated")
	}

	// A completed run must not carry an error message.
	rec2, err := normalizeRunRecord(RunRecord{
		CompanyID:     "c1",
		RetentionDays: 30,
		Cutoff:        cutoff,
		Status:        RunStatusCompleted,
		ErrorMessage:  "should be cleared",
	}, now)
	if err != nil {
		t.Fatalf("normalizeRunRecord: %v", err)
	}
	if rec2.ErrorMessage != "" {
		t.Fatalf("completed run error message = %q, want empty", rec2.ErrorMessage)
	}

	// A failed run keeps a cleaned error message.
	rec3, err := normalizeRunRecord(RunRecord{
		CompanyID:     "c1",
		RetentionDays: 30,
		Cutoff:        cutoff,
		Status:        RunStatusFailed,
		ErrorMessage:  "boom\nnewline",
	}, now)
	if err != nil {
		t.Fatalf("normalizeRunRecord: %v", err)
	}
	if strings.ContainsAny(rec3.ErrorMessage, "\n\r") || rec3.ErrorMessage == "" {
		t.Fatalf("failed run error message = %q, want cleaned non-empty", rec3.ErrorMessage)
	}
}

func TestValidateRunID(t *testing.T) {
	t.Parallel()

	id, err := validateRunID("  audit-retention-abc  ")
	if err != nil || id != "audit-retention-abc" {
		t.Fatalf("validateRunID trim = (%q, %v)", id, err)
	}
	for _, bad := range []string{"", "audit\nretention", strings.Repeat("x", maxRunIDBytes+1)} {
		if _, err := validateRunID(bad); err == nil {
			t.Errorf("validateRunID(%q) = nil, want error", bad)
		}
	}
}

func TestRepositoryNilDBGuards(t *testing.T) {
	t.Parallel()

	repo := &Repository{}
	ctx := t.Context()
	if _, err := repo.GetPolicy(ctx, "c1"); err == nil {
		t.Error("GetPolicy nil db: want error")
	}
	if _, err := repo.SetPolicy(ctx, "c1", 30, ""); err == nil {
		t.Error("SetPolicy nil db: want error")
	}
	if err := repo.DeletePolicy(ctx, "c1"); err == nil {
		t.Error("DeletePolicy nil db: want error")
	}
	if _, err := repo.ListPolicies(ctx); err == nil {
		t.Error("ListPolicies nil db: want error")
	}
	if _, err := repo.PurgeExpired(ctx, "c1", time.Now(), 100); err == nil {
		t.Error("PurgeExpired nil db: want error")
	}
	if _, err := repo.CountExpired(ctx, "c1", time.Now()); err == nil {
		t.Error("CountExpired nil db: want error")
	}
	if _, err := repo.RecordRun(ctx, RunRecord{}); err == nil {
		t.Error("RecordRun nil db: want error")
	}
	if _, err := repo.ListRuns(ctx, RunListRequest{CompanyID: "c1"}); err == nil {
		t.Error("ListRuns nil db: want error")
	}
	// SetPolicy must reject the forever sentinel (callers use DeletePolicy).
	repoWithNoDBButValidation := &Repository{}
	if _, err := repoWithNoDBButValidation.SetPolicy(ctx, "c1", RetentionForever, ""); err == nil {
		t.Error("SetPolicy(forever) : want error")
	}
}
