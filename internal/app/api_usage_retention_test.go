package app

import (
	"context"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/maildb"
)

type fakeAPIUsageRetentionRunner struct {
	lastRequest maildb.APIUsageLedgerRetentionRunRequest
	run         maildb.APIUsageLedgerRetentionRunView
	err         error
}

func (f *fakeAPIUsageRetentionRunner) RunAPIUsageLedgerRetention(_ context.Context, req maildb.APIUsageLedgerRetentionRunRequest) (maildb.APIUsageLedgerRetentionRunView, error) {
	f.lastRequest = req
	if f.err != nil {
		return maildb.APIUsageLedgerRetentionRunView{}, f.err
	}
	return f.run, nil
}

func TestRunAPIUsageRetentionOnceBuildsGuardedRequest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.FixedZone("KST", 9*60*60))
	runner := &fakeAPIUsageRetentionRunner{
		run: maildb.APIUsageLedgerRetentionRunView{
			ID:             "api-usage-retention-1",
			CandidateCount: 12,
			LimitedCount:   5,
			DeletedCount:   0,
			Ready:          true,
			DryRun:         true,
		},
	}
	cfg := config.Config{
		APIUsageRetentionCutoffAge:    30 * 24 * time.Hour,
		APIUsageRetentionBatchSize:    5,
		APIUsageRetentionDryRun:       true,
		APIUsageRetentionConfirmReady: false,
		APIUsageRetentionTenantID:     "tenant-1",
		APIUsageRetentionPrincipalID:  "principal-1",
	}

	result, err := runAPIUsageRetentionOnce(context.Background(), runner, func() time.Time {
		return now
	}, cfg, nil)
	if err != nil {
		t.Fatalf("runAPIUsageRetentionOnce returned error: %v", err)
	}
	if result.RunID != "api-usage-retention-1" || result.CandidateCount != 12 || result.LimitedCount != 5 || result.DeletedCount != 0 || !result.DryRun {
		t.Fatalf("result = %+v", result)
	}
	wantCutoff := now.UTC().Add(-30 * 24 * time.Hour)
	if !runner.lastRequest.Cutoff.Equal(wantCutoff) {
		t.Fatalf("cutoff = %s, want %s", runner.lastRequest.Cutoff, wantCutoff)
	}
	if runner.lastRequest.Limit != 5 ||
		!runner.lastRequest.DryRun ||
		runner.lastRequest.ConfirmReady ||
		runner.lastRequest.TenantID != "tenant-1" ||
		runner.lastRequest.PrincipalID != "principal-1" {
		t.Fatalf("request = %+v", runner.lastRequest)
	}
}
