package apimeter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const ExportManifestSchemaV1 = "2026-05-04.api-usage-export-manifest.v1"

type ExportManifest struct {
	SchemaVersion string                   `json:"schema_version"`
	Batch         ExportManifestBatch      `json:"batch"`
	Artifacts     []ExportManifestArtifact `json:"artifacts"`
}

type ExportManifestBatch struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenant_id,omitempty"`
	PrincipalID    string `json:"principal_id,omitempty"`
	WindowStart    string `json:"window_start,omitempty"`
	WindowEnd      string `json:"window_end,omitempty"`
	EventCount     int64  `json:"event_count"`
	RequestCount   int64  `json:"request_count"`
	RequestBytes   int64  `json:"request_bytes"`
	ResponseBytes  int64  `json:"response_bytes"`
	LatencyMSTotal int64  `json:"latency_ms_total"`
	LatencyMSMax   int64  `json:"latency_ms_max"`
}

type ExportManifestArtifact struct {
	ID             string `json:"id"`
	StorageBackend string `json:"storage_backend"`
	ObjectKey      string `json:"object_key"`
	ContentType    string `json:"content_type"`
	ByteCount      int64  `json:"byte_count"`
	SHA256Hex      string `json:"sha256_hex"`
	EventCount     int64  `json:"event_count"`
}

func DigestExportManifest(manifest ExportManifest) (string, []byte, error) {
	if manifest.SchemaVersion == "" {
		manifest.SchemaVersion = ExportManifestSchemaV1
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return "", nil, fmt.Errorf("marshal api usage export manifest: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), raw, nil
}

func FormatManifestTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
