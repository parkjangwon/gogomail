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

func TestAPIUsageExportArtifactsQueryLimitModes(t *testing.T) {
	t.Parallel()

	if got := apiUsageExportArtifactsQuery(false); !strings.Contains(got, "LIMIT $2") {
		t.Fatalf("bounded query missing limit: %s", got)
	}
	if got := apiUsageExportArtifactsQuery(true); strings.Contains(got, "LIMIT") {
		t.Fatalf("unbounded query should not include limit: %s", got)
	}
}

func TestAPIUsageExportArtifactAuditDetail(t *testing.T) {
	t.Parallel()

	detail, err := apiUsageExportArtifactAuditDetail(APIUsageExportArtifactView{
		ID:             "11111111-1111-1111-1111-111111111111",
		BatchID:        "22222222-2222-2222-2222-222222222222",
		StorageBackend: "external",
		ObjectKey:      "exports/api-usage.ndjson",
		ContentType:    "application/x-ndjson",
		ByteCount:      2048,
		SHA256Hex:      strings.Repeat("a", 64),
		EventCount:     100,
		Metadata:       json.RawMessage(`{"ignored":"metadata is not audit detail"}`),
	})
	if err != nil {
		t.Fatalf("apiUsageExportArtifactAuditDetail returned error: %v", err)
	}
	var got struct {
		ArtifactID     string `json:"artifact_id"`
		BatchID        string `json:"batch_id"`
		StorageBackend string `json:"storage_backend"`
		ObjectKey      string `json:"object_key"`
		ContentType    string `json:"content_type"`
		ByteCount      int64  `json:"byte_count"`
		SHA256Hex      string `json:"sha256_hex"`
		EventCount     int64  `json:"event_count"`
	}
	if err := json.Unmarshal(detail, &got); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if got.ArtifactID == "" || got.BatchID == "" || got.StorageBackend != "external" || got.ObjectKey != "exports/api-usage.ndjson" {
		t.Fatalf("audit detail identity = %+v", got)
	}
	if got.ByteCount != 2048 || got.SHA256Hex != strings.Repeat("a", 64) || got.EventCount != 100 {
		t.Fatalf("audit detail metrics = %+v", got)
	}
	if strings.Contains(string(detail), "metadata") {
		t.Fatalf("audit detail leaked metadata: %s", detail)
	}
}
