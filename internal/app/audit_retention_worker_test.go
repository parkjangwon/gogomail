package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/auditretention"
	"github.com/gogomail/gogomail/internal/config"
)

// fakeAuditRetentionPurger simulates the auditretention repository. It models a
// per-company store of audit log timestamps so purge/count behavior can be
// asserted precisely.
type fakeAuditRetentionPurger struct {
	policies []auditretention.Policy
	// logs maps companyID -> audit log creation times.
	logs map[string][]time.Time
	// recorded captures every RecordRun call.
	recorded []auditretention.RunRecord
	// purgeErr, when set for a company, forces PurgeExpired to fail.
	purgeErr map[string]error
	// listErr forces ListPolicies to fail.
	listErr error
}

func (f *fakeAuditRetentionPurger) ListPolicies(context.Context) ([]auditretention.Policy, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.policies, nil
}

func (f *fakeAuditRetentionPurger) countBefore(companyID string, cutoff time.Time) int64 {
	var n int64
	for _, ts := range f.logs[companyID] {
		if ts.Before(cutoff) {
			n++
		}
	}
	return n
}

func (f *fakeAuditRetentionPurger) PurgeExpired(_ context.Context, companyID string, cutoff time.Time, batchSize int) (int64, error) {
	if err := f.purgeErr[companyID]; err != nil {
		return 0, err
	}
	if batchSize <= 0 {
		return 0, fmt.Errorf("batch size must be positive")
	}
	remaining := f.logs[companyID][:0]
	var deleted int64
	for _, ts := range f.logs[companyID] {
		if ts.Before(cutoff) {
			deleted++
			continue
		}
		remaining = append(remaining, ts)
	}
	f.logs[companyID] = remaining
	return deleted, nil
}

func (f *fakeAuditRetentionPurger) CountExpired(_ context.Context, companyID string, cutoff time.Time) (int64, error) {
	return f.countBefore(companyID, cutoff), nil
}

func (f *fakeAuditRetentionPurger) RecordRun(_ context.Context, record auditretention.RunRecord) (auditretention.RunRecord, error) {
	if record.ID == "" {
		record.ID = fmt.Sprintf("audit-retention-%d", len(f.recorded))
	}
	f.recorded = append(f.recorded, record)
	return record, nil
}

func fixedNow(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestRunAuditRetentionPurgeOnceDeletesExpiredKeepsRecent(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// company A: 90-day policy. Two old (>90d) rows, one recent row.
	repo := &fakeAuditRetentionPurger{
		policies: []auditretention.Policy{
			{CompanyID: "company-a", RetentionDays: 90},
		},
		logs: map[string][]time.Time{
			"company-a": {
				now.Add(-200 * 24 * time.Hour), // expired
				now.Add(-100 * 24 * time.Hour), // expired
				now.Add(-10 * 24 * time.Hour),  // kept
			},
			// company with no policy — must remain untouched.
			"company-b": {
				now.Add(-500 * 24 * time.Hour),
			},
		},
	}
	cfg := config.Config{AuditRetentionBatchSize: 1000}

	result, err := runAuditRetentionPurgeOnce(context.Background(), repo, fixedNow(now), cfg, nil)
	if err != nil {
		t.Fatalf("runAuditRetentionPurgeOnce: %v", err)
	}
	if result.CompaniesScanned != 1 {
		t.Fatalf("companies scanned = %d, want 1", result.CompaniesScanned)
	}
	if result.RowsDeleted != 2 {
		t.Fatalf("rows deleted = %d, want 2", result.RowsDeleted)
	}
	// The recent row for company-a must remain.
	if got := len(repo.logs["company-a"]); got != 1 {
		t.Fatalf("company-a remaining logs = %d, want 1 (recent kept)", got)
	}
	// company-b has no policy: never scanned, never purged.
	if got := len(repo.logs["company-b"]); got != 1 {
		t.Fatalf("company-b logs = %d, want 1 (no policy, untouched)", got)
	}
	// A run must be recorded, completed, not dry-run.
	if len(repo.recorded) != 1 {
		t.Fatalf("recorded runs = %d, want 1", len(repo.recorded))
	}
	run := repo.recorded[0]
	if run.CompanyID != "company-a" || run.RetentionDays != 90 || run.RowsDeleted != 2 {
		t.Fatalf("recorded run = %+v", run)
	}
	if run.DryRun {
		t.Fatal("recorded run dry_run = true, want false")
	}
	if run.Status != auditretention.RunStatusCompleted {
		t.Fatalf("recorded run status = %q, want completed", run.Status)
	}
	wantCutoff := now.Add(-90 * 24 * time.Hour)
	if !run.Cutoff.Equal(wantCutoff) {
		t.Fatalf("recorded cutoff = %s, want %s", run.Cutoff, wantCutoff)
	}
}

func TestRunAuditRetentionPurgeOnceDryRunCountsWithoutDeleting(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakeAuditRetentionPurger{
		policies: []auditretention.Policy{{CompanyID: "company-a", RetentionDays: 30}},
		logs: map[string][]time.Time{
			"company-a": {
				now.Add(-60 * 24 * time.Hour),
				now.Add(-40 * 24 * time.Hour),
				now.Add(-5 * 24 * time.Hour),
			},
		},
	}
	cfg := config.Config{AuditRetentionBatchSize: 1000, AuditRetentionDryRun: true}

	result, err := runAuditRetentionPurgeOnce(context.Background(), repo, fixedNow(now), cfg, nil)
	if err != nil {
		t.Fatalf("runAuditRetentionPurgeOnce: %v", err)
	}
	// Dry-run reports what would be deleted (2) but deletes nothing.
	if result.RowsDeleted != 2 {
		t.Fatalf("dry-run rows would-delete = %d, want 2", result.RowsDeleted)
	}
	if got := len(repo.logs["company-a"]); got != 3 {
		t.Fatalf("dry-run must not delete: remaining = %d, want 3", got)
	}
	if len(repo.recorded) != 1 || !repo.recorded[0].DryRun || repo.recorded[0].RowsDeleted != 2 {
		t.Fatalf("recorded dry-run = %+v", repo.recorded)
	}
}

func TestRunAuditRetentionPurgeOnceNoPolicies(t *testing.T) {
	repo := &fakeAuditRetentionPurger{logs: map[string][]time.Time{}}
	cfg := config.Config{AuditRetentionBatchSize: 1000}
	result, err := runAuditRetentionPurgeOnce(context.Background(), repo, fixedNow(time.Now()), cfg, nil)
	if err != nil {
		t.Fatalf("runAuditRetentionPurgeOnce: %v", err)
	}
	if result.CompaniesScanned != 0 || result.RowsDeleted != 0 || len(repo.recorded) != 0 {
		t.Fatalf("no-policy sweep result = %+v, recorded = %d", result, len(repo.recorded))
	}
}

func TestRunAuditRetentionPurgeOnceRecordsFailure(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakeAuditRetentionPurger{
		policies: []auditretention.Policy{{CompanyID: "company-a", RetentionDays: 30}},
		logs:     map[string][]time.Time{"company-a": {now.Add(-60 * 24 * time.Hour)}},
		purgeErr: map[string]error{"company-a": fmt.Errorf("db exploded")},
	}
	cfg := config.Config{AuditRetentionBatchSize: 1000}

	_, err := runAuditRetentionPurgeOnce(context.Background(), repo, fixedNow(now), cfg, nil)
	if err == nil {
		t.Fatal("runAuditRetentionPurgeOnce: want error on purge failure")
	}
	// The failed run must still be recorded for operator visibility.
	if len(repo.recorded) != 1 {
		t.Fatalf("recorded runs = %d, want 1 (failure recorded)", len(repo.recorded))
	}
	run := repo.recorded[0]
	if run.Status != auditretention.RunStatusFailed || run.RowsDeleted != 0 || run.ErrorMessage == "" {
		t.Fatalf("recorded failure run = %+v", run)
	}
}

func TestRunAuditRetentionPurgeOnceNilRepo(t *testing.T) {
	if _, err := runAuditRetentionPurgeOnce(context.Background(), nil, time.Now, config.Config{}, nil); err == nil {
		t.Fatal("runAuditRetentionPurgeOnce nil repo: want error")
	}
}
