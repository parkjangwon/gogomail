package maildb

import (
	"context"
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
