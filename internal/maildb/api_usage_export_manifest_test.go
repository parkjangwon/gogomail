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
