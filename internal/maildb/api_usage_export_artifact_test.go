package maildb

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateCreateAPIUsageExportArtifactRequestDefaultsAndNormalizes(t *testing.T) {
	t.Parallel()

	req := CreateAPIUsageExportArtifactRequest{
		BatchID:   " batch-1 ",
		ObjectKey: " exports/batch-1.ndjson ",
		ByteCount: 12,
		SHA256Hex: strings.Repeat("A", 64),
		Metadata:  json.RawMessage(`{"writer":"test"}`),
	}
	if err := ValidateCreateAPIUsageExportArtifactRequest(&req); err != nil {
		t.Fatalf("ValidateCreateAPIUsageExportArtifactRequest returned error: %v", err)
	}
	if req.BatchID != "batch-1" || req.ObjectKey != "exports/batch-1.ndjson" {
		t.Fatalf("normalized request = %+v", req)
	}
	if req.StorageBackend != "external" || req.ContentType != "application/x-ndjson" {
		t.Fatalf("defaults = %+v", req)
	}
	if req.SHA256Hex != strings.Repeat("a", 64) {
		t.Fatalf("SHA256Hex = %q", req.SHA256Hex)
	}
}

func TestValidateCreateAPIUsageExportArtifactRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	validHash := strings.Repeat("a", 64)
	tests := []struct {
		name string
		req  CreateAPIUsageExportArtifactRequest
	}{
		{name: "missing batch", req: CreateAPIUsageExportArtifactRequest{ObjectKey: "x", SHA256Hex: validHash}},
		{name: "missing object", req: CreateAPIUsageExportArtifactRequest{BatchID: "batch-1", SHA256Hex: validHash}},
		{name: "line break object", req: CreateAPIUsageExportArtifactRequest{BatchID: "batch-1", ObjectKey: "x\n", SHA256Hex: validHash}},
		{name: "bad content type", req: CreateAPIUsageExportArtifactRequest{BatchID: "batch-1", ObjectKey: "x", ContentType: "text/plain", SHA256Hex: validHash}},
		{name: "negative bytes", req: CreateAPIUsageExportArtifactRequest{BatchID: "batch-1", ObjectKey: "x", ByteCount: -1, SHA256Hex: validHash}},
		{name: "bad hash", req: CreateAPIUsageExportArtifactRequest{BatchID: "batch-1", ObjectKey: "x", SHA256Hex: "nope"}},
		{name: "array metadata", req: CreateAPIUsageExportArtifactRequest{BatchID: "batch-1", ObjectKey: "x", SHA256Hex: validHash, Metadata: json.RawMessage(`[]`)}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateCreateAPIUsageExportArtifactRequest(&tc.req); err == nil {
				t.Fatalf("ValidateCreateAPIUsageExportArtifactRequest(%+v) returned nil", tc.req)
			}
		})
	}
}
