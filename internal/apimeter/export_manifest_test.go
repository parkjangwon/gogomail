package apimeter

import (
	"strings"
	"testing"
	"time"
)

func TestDigestExportManifestIsStable(t *testing.T) {
	t.Parallel()

	manifest := ExportManifest{
		Batch: ExportManifestBatch{
			ID:           "batch-1",
			TenantID:     "tenant-1",
			EventCount:   2,
			RequestCount: 2,
		},
		Artifacts: []ExportManifestArtifact{{
			ID:          "artifact-1",
			ObjectKey:   "exports/batch-1.ndjson",
			ContentType: "application/x-ndjson",
			ByteCount:   12,
			SHA256Hex:   strings.Repeat("a", 64),
			EventCount:  2,
		}},
	}

	first, firstRaw, err := DigestExportManifest(manifest)
	if err != nil {
		t.Fatalf("DigestExportManifest returned error: %v", err)
	}
	second, secondRaw, err := DigestExportManifest(manifest)
	if err != nil {
		t.Fatalf("DigestExportManifest returned error: %v", err)
	}
	if first != second || string(firstRaw) != string(secondRaw) {
		t.Fatalf("digest/raw changed: %q/%s vs %q/%s", first, firstRaw, second, secondRaw)
	}
	if len(first) != 64 {
		t.Fatalf("digest length = %d, want 64", len(first))
	}
	if !strings.Contains(string(firstRaw), ExportManifestSchemaV1) {
		t.Fatalf("manifest raw missing schema version: %s", firstRaw)
	}
}

func TestDigestExportManifestJSONCanonicalizesInput(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"artifacts":[{"event_count":2,"sha256_hex":"` + strings.Repeat("a", 64) + `","byte_count":12,"content_type":"application/x-ndjson","object_key":"exports/batch-1.ndjson","id":"artifact-1"}],
		"batch":{"request_count":2,"event_count":2,"tenant_id":"tenant-1","id":"batch-1"},
		"schema_version":"2026-05-04.api-usage-export-manifest.v1"
	}`)

	fromJSON, canonical, err := DigestExportManifestJSON(raw)
	if err != nil {
		t.Fatalf("DigestExportManifestJSON returned error: %v", err)
	}
	fromStruct, expectedCanonical, err := DigestExportManifest(ExportManifest{
		SchemaVersion: ExportManifestSchemaV1,
		Batch: ExportManifestBatch{
			ID:           "batch-1",
			TenantID:     "tenant-1",
			EventCount:   2,
			RequestCount: 2,
		},
		Artifacts: []ExportManifestArtifact{{
			ID:          "artifact-1",
			ObjectKey:   "exports/batch-1.ndjson",
			ContentType: "application/x-ndjson",
			ByteCount:   12,
			SHA256Hex:   strings.Repeat("a", 64),
			EventCount:  2,
		}},
	})
	if err != nil {
		t.Fatalf("DigestExportManifest returned error: %v", err)
	}
	if fromJSON != fromStruct || string(canonical) != string(expectedCanonical) {
		t.Fatalf("canonical digest/raw mismatch: %q/%s vs %q/%s", fromJSON, canonical, fromStruct, expectedCanonical)
	}
}

func TestDigestExportManifestRejectsUnsupportedSchema(t *testing.T) {
	t.Parallel()

	_, _, err := DigestExportManifest(ExportManifest{
		SchemaVersion: "2099-01-01.api-usage-export-manifest.v9",
		Batch:         ExportManifestBatch{ID: "batch-1"},
	})
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("err = %v, want schema_version error", err)
	}
}

func TestFormatManifestTime(t *testing.T) {
	t.Parallel()

	value := time.Date(2026, 5, 4, 9, 30, 0, 123, time.FixedZone("KST", 9*60*60))
	if got := FormatManifestTime(&value); got != "2026-05-04T00:30:00.000000123Z" {
		t.Fatalf("FormatManifestTime = %q", got)
	}
	if got := FormatManifestTime(nil); got != "" {
		t.Fatalf("FormatManifestTime(nil) = %q", got)
	}
}
