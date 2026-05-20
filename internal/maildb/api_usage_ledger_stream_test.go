package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestStreamAPIUsageLedgerRejectsNilDatabase(t *testing.T) {
	t.Parallel()

	err := (&Repository{}).StreamAPIUsageLedger(context.Background(), APIUsageLedgerListRequest{}, nil)
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestGetAPIUsageLedgerRetentionReadinessRejectsNilDatabase(t *testing.T) {
	t.Parallel()

	errView, err := (&Repository{}).GetAPIUsageLedgerRetentionReadiness(context.Background(), APIUsageLedgerRetentionRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("view = %+v err = %v", errView, err)
	}
}

func TestRunAPIUsageLedgerRetentionRejectsNilDatabase(t *testing.T) {
	t.Parallel()

	view, err := (&Repository{}).RunAPIUsageLedgerRetention(context.Background(), APIUsageLedgerRetentionRunRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("view = %+v err = %v", view, err)
	}
}

func TestAPIUsageLedgerRetentionRejectsFutureCutoff(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("pgx", "postgres://gogomail.invalid/gogomail")
	if err != nil {
		t.Fatalf("open db handle: %v", err)
	}
	defer db.Close()
	repo := NewRepository(db)
	future := time.Now().UTC().Add(time.Hour)
	readiness, err := repo.GetAPIUsageLedgerRetentionReadiness(context.Background(), APIUsageLedgerRetentionRequest{Cutoff: future})
	if err == nil || !strings.Contains(err.Error(), "cutoff must not be in the future") {
		t.Fatalf("readiness = %+v err = %v", readiness, err)
	}
	run, err := repo.RunAPIUsageLedgerRetention(context.Background(), APIUsageLedgerRetentionRunRequest{
		Cutoff:       future,
		DryRun:       true,
		ConfirmReady: false,
	})
	if err == nil || !strings.Contains(err.Error(), "cutoff must not be in the future") {
		t.Fatalf("run = %+v err = %v", run, err)
	}
}

func TestAPIUsageLedgerRetentionRunReadsRejectNilDatabase(t *testing.T) {
	t.Parallel()

	runs, err := (&Repository{}).ListAPIUsageLedgerRetentionRuns(context.Background(), APIUsageLedgerRetentionRunListRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("runs = %+v err = %v", runs, err)
	}
	view, err := (&Repository{}).GetAPIUsageLedgerRetentionRun(context.Background(), "api-usage-retention-1")
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("view = %+v err = %v", view, err)
	}
}

func TestAPIUsageLedgerRetentionRunAuditDetail(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	detail, err := apiUsageLedgerRetentionRunAuditDetail(APIUsageLedgerRetentionRunView{
		ID:             "api-usage-retention-1",
		Cutoff:         time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		TenantID:       "tenant-1",
		PrincipalID:    "principal-1",
		Limit:          100,
		DryRun:         false,
		ConfirmReady:   true,
		Ready:          true,
		CandidateCount: 200,
		LimitedCount:   100,
		DeletedCount:   100,
		Readiness: APIUsageLedgerRetentionReadinessView{
			BlockingReasons:                []string{},
			CoveringExportBatchID:          "batch-1",
			CoveringExportBatchCompletedAt: &completedAt,
			CoveringArtifactCount:          1,
			CoveringManifestDigestCount:    1,
			CoveringManifestSignatureCount: 1,
			CandidateRequestCount:          123,
			CandidateRequestBytes:          456,
			CandidateResponseBytes:         789,
			CandidateLatencyMSTotal:        321,
			CandidateLatencyMSMax:          10,
			CoveringExportBatchEventCount:  200,
			CoveringArtifactEventCount:     200,
		},
	})
	if err != nil {
		t.Fatalf("apiUsageLedgerRetentionRunAuditDetail returned error: %v", err)
	}
	var got struct {
		RunID                          string   `json:"run_id"`
		Cutoff                         string   `json:"cutoff"`
		TenantID                       string   `json:"tenant_id"`
		PrincipalID                    string   `json:"principal_id"`
		Limit                          int      `json:"limit"`
		DryRun                         bool     `json:"dry_run"`
		ConfirmReady                   bool     `json:"confirm_ready"`
		Ready                          bool     `json:"ready"`
		CandidateCount                 int64    `json:"candidate_count"`
		LimitedCount                   int64    `json:"limited_count"`
		DeletedCount                   int64    `json:"deleted_count"`
		BlockingReasons                []string `json:"blocking_reasons"`
		CoveringExportBatchID          string   `json:"covering_export_batch_id"`
		CoveringManifestSignatureCount int64    `json:"covering_manifest_signature_count"`
		CoveringExportBatchCompletedAt string   `json:"covering_export_batch_completed_at"`
	}
	if err := json.Unmarshal(detail, &got); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if got.RunID != "api-usage-retention-1" || got.Cutoff != "2026-05-01T00:00:00Z" || got.TenantID != "tenant-1" || got.PrincipalID != "principal-1" {
		t.Fatalf("audit detail identity = %+v", got)
	}
	if got.Limit != 100 || got.DryRun || !got.ConfirmReady || !got.Ready || got.CandidateCount != 200 || got.LimitedCount != 100 || got.DeletedCount != 100 {
		t.Fatalf("audit detail run fields = %+v", got)
	}
	if got.CoveringExportBatchID != "batch-1" || got.CoveringManifestSignatureCount != 1 || got.CoveringExportBatchCompletedAt != "2026-05-04T12:00:00Z" {
		t.Fatalf("audit detail evidence = %+v", got)
	}
	if strings.Contains(string(detail), "candidate_request_bytes") || strings.Contains(string(detail), "readiness") {
		t.Fatalf("audit detail leaked full readiness payload: %s", detail)
	}
}

func TestAPIUsageLedgerRetentionRunAuditResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		view APIUsageLedgerRetentionRunView
		want string
	}{
		{name: "dry run", view: APIUsageLedgerRetentionRunView{DryRun: true}, want: "dry_run"},
		{name: "blocked", view: APIUsageLedgerRetentionRunView{Ready: false}, want: "blocked"},
		{name: "no op", view: APIUsageLedgerRetentionRunView{Ready: true}, want: "no_op"},
		{name: "completed", view: APIUsageLedgerRetentionRunView{Ready: true, DeletedCount: 1}, want: "completed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := apiUsageLedgerRetentionRunAuditResult(tc.view); got != tc.want {
				t.Fatalf("result = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAuditLogReadsRejectNilDatabase(t *testing.T) {
	t.Parallel()

	logs, _, err := (&Repository{}).ListAuditLogs(context.Background(), AuditLogListRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("logs = %+v err = %v", logs, err)
	}
	view, err := (&Repository{}).GetAuditLog(context.Background(), "audit-1")
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("view = %+v err = %v", view, err)
	}
	integrity, err := (&Repository{}).CheckAuditLogIntegrity(context.Background(), AuditLogIntegrityRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("integrity = %+v err = %v", integrity, err)
	}
}

func TestNormalizeAPIUsageLedgerRetentionLimit(t *testing.T) {
	t.Parallel()

	tests := map[int]int{
		0:                                       APIUsageLedgerRetentionDefaultLimit,
		-1:                                      APIUsageLedgerRetentionDefaultLimit,
		25:                                      25,
		APIUsageLedgerRetentionMaxLimit:         APIUsageLedgerRetentionMaxLimit,
		APIUsageLedgerRetentionMaxLimit + 1:     APIUsageLedgerRetentionMaxLimit,
		APIUsageLedgerRetentionDefaultLimit + 1: APIUsageLedgerRetentionDefaultLimit + 1,
	}
	for input, want := range tests {
		if got := NormalizeAPIUsageLedgerRetentionLimit(input); got != want {
			t.Fatalf("NormalizeAPIUsageLedgerRetentionLimit(%d) = %d, want %d", input, got, want)
		}
	}
}

func TestAPIUsageLedgerStreamLimit(t *testing.T) {
	t.Parallel()

	limit, unbounded := apiUsageLedgerStreamLimit(APIUsageLedgerNoLimit)
	if limit != 0 || !unbounded {
		t.Fatalf("no-limit = %d/%v", limit, unbounded)
	}

	limit, unbounded = apiUsageLedgerStreamLimit(0)
	if limit != MessageListDefaultLimit || unbounded {
		t.Fatalf("default limit = %d/%v", limit, unbounded)
	}

	limit, unbounded = apiUsageLedgerStreamLimit(MessageListMaxLimit + 1)
	if limit != MessageListMaxLimit || unbounded {
		t.Fatalf("max limit = %d/%v", limit, unbounded)
	}
}

func TestApplyAPIUsageLedgerRetentionReadiness(t *testing.T) {
	t.Parallel()

	view := APIUsageLedgerRetentionReadinessView{CandidateEventCount: 10}
	applyAPIUsageLedgerRetentionReadiness(&view)
	if view.Ready || strings.Join(view.BlockingReasons, ",") != "covering_export_batch_required" {
		t.Fatalf("retention readiness = %+v", view)
	}

	view.CoveringExportBatchID = "api-usage-export-1"
	view.CoveringExportBatchEventCount = 10
	view.CoveringArtifactCount = 1
	view.CoveringArtifactEventCount = 10
	view.CoveringManifestDigestCount = 1
	view.CoveringManifestSignatureCount = 1
	applyAPIUsageLedgerRetentionReadiness(&view)
	if !view.Ready || len(view.BlockingReasons) != 0 {
		t.Fatalf("retention readiness = %+v", view)
	}
}

func TestApplyAPIUsageLedgerRetentionReadinessBlocksWeakEvidence(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, 5, 4, 1, 0, 0, 0, time.UTC)
	latestRecordedAt := time.Date(2026, 5, 4, 2, 0, 0, 0, time.UTC)
	view := APIUsageLedgerRetentionReadinessView{
		CandidateEventCount:            10,
		LatestCandidateRecordedAt:      &latestRecordedAt,
		CoveringExportBatchID:          "api-usage-export-1",
		CoveringExportBatchCompletedAt: &completedAt,
		CoveringExportBatchEventCount:  10,
		CoveringArtifactCount:          1,
		CoveringArtifactEventCount:     9,
	}
	applyAPIUsageLedgerRetentionReadiness(&view)

	want := "covering_export_batch_stale,covering_export_artifact_required,covering_manifest_digest_required,covering_manifest_signature_required"
	if view.Ready || strings.Join(view.BlockingReasons, ",") != want {
		t.Fatalf("retention readiness = %+v, want blocking %s", view, want)
	}
}
