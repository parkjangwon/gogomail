package maildb

import (
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/apimeter"
)

func TestAPIUsageExportManifestSortsArtifacts(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	manifest := apiUsageExportManifest(
		APIUsageExportBatchView{
			ID:           "batch-1",
			TenantID:     "tenant-1",
			WindowStart:  &start,
			WindowEnd:    &end,
			EventCount:   2,
			RequestCount: 2,
		},
		[]APIUsageExportArtifactView{
			{ID: "artifact-b", ObjectKey: "b.ndjson", ContentType: "application/x-ndjson", SHA256Hex: strings.Repeat("b", 64)},
			{ID: "artifact-a", ObjectKey: "a.ndjson", ContentType: "application/x-ndjson", SHA256Hex: strings.Repeat("a", 64)},
		},
	)

	if manifest.SchemaVersion != apimeter.ExportManifestSchemaV1 {
		t.Fatalf("SchemaVersion = %q", manifest.SchemaVersion)
	}
	if manifest.Batch.WindowStart != "2026-05-04T00:00:00Z" || manifest.Batch.WindowEnd != "2026-05-05T00:00:00Z" {
		t.Fatalf("manifest batch = %+v", manifest.Batch)
	}
	if len(manifest.Artifacts) != 2 || manifest.Artifacts[0].ID != "artifact-a" || manifest.Artifacts[1].ID != "artifact-b" {
		t.Fatalf("artifacts = %+v", manifest.Artifacts)
	}
}

func TestAPIUsageExportManifestDoesNotMutateArtifacts(t *testing.T) {
	t.Parallel()

	artifacts := []APIUsageExportArtifactView{
		{ID: "artifact-b", SHA256Hex: strings.Repeat("b", 64)},
		{ID: "artifact-a", SHA256Hex: strings.Repeat("a", 64)},
	}
	_ = apiUsageExportManifest(APIUsageExportBatchView{ID: "batch-1"}, artifacts)

	if artifacts[0].ID != "artifact-b" || artifacts[1].ID != "artifact-a" {
		t.Fatalf("artifacts were mutated: %+v", artifacts)
	}
}

func TestAPIUsageExportManifestDigestVerification(t *testing.T) {
	t.Parallel()

	manifest := apimeter.ExportManifest{
		SchemaVersion: apimeter.ExportManifestSchemaV1,
		Batch: apimeter.ExportManifestBatch{
			ID:           "batch-1",
			EventCount:   2,
			RequestCount: 2,
		},
	}
	digestHex, raw, err := apimeter.DigestExportManifest(manifest)
	if err != nil {
		t.Fatalf("DigestExportManifest returned error: %v", err)
	}

	verification, err := apiUsageExportManifestDigestVerification(APIUsageExportManifestDigestView{
		ID:              "digest-1",
		BatchID:         "batch-1",
		SchemaVersion:   apimeter.ExportManifestSchemaV1,
		DigestAlgorithm: "sha256",
		DigestHex:       digestHex,
		Manifest:        raw,
	})
	if err != nil {
		t.Fatalf("apiUsageExportManifestDigestVerification returned error: %v", err)
	}
	if !verification.Valid || verification.ActualDigestHex != digestHex || len(verification.CanonicalManifest) == 0 {
		t.Fatalf("verification = %+v", verification)
	}
}

func TestApplyAPIUsageExportHandoffReadiness(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	view := APIUsageExportHandoffView{
		BatchID:                    "batch-1",
		BatchStatus:                "completed",
		BatchCompleted:             true,
		EventCount:                 10,
		ArtifactCount:              1,
		ArtifactEventCount:         10,
		ManifestDigestCount:        1,
		LatestManifestDigestID:     "digest-1",
		LatestManifestDigestAt:     &completedAt,
		LatestDigestSignatureCount: 1,
		LatestSignatureID:          "signature-1",
		LatestSignatureAt:          &completedAt,
	}
	applyAPIUsageExportHandoffReadiness(&view)

	if !view.Ready || !view.EventsCovered || len(view.MissingRequirements) != 0 {
		t.Fatalf("handoff readiness = %+v", view)
	}
}

func TestApplyAPIUsageExportHandoffReadinessReportsMissingRequirements(t *testing.T) {
	t.Parallel()

	view := APIUsageExportHandoffView{
		BatchID:                "batch-1",
		BatchStatus:            "completed",
		EventCount:             10,
		ArtifactCount:          1,
		ArtifactEventCount:     9,
		ManifestDigestCount:    1,
		LatestManifestDigestID: "digest-1",
	}
	applyAPIUsageExportHandoffReadiness(&view)

	want := []string{"batch_completed", "event_coverage", "manifest_signature"}
	if view.Ready || view.EventsCovered || strings.Join(view.MissingRequirements, ",") != strings.Join(want, ",") {
		t.Fatalf("handoff readiness = %+v, want missing %v", view, want)
	}
}
