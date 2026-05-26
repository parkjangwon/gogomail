package maildb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/audit"
)

func (r *Repository) CreateAPIUsageExportArtifact(ctx context.Context, req CreateAPIUsageExportArtifactRequest) (APIUsageExportArtifactView, error) {
	if r.db == nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateAPIUsageExportArtifactRequest(&req); err != nil {
		return APIUsageExportArtifactView{}, err
	}
	id, err := newAPIUsageExportArtifactID()
	if err != nil {
		return APIUsageExportArtifactView{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("begin api usage export artifact transaction: %w", err)
	}
	defer tx.Rollback()
	const query = `
INSERT INTO api_usage_export_artifacts (
  id,
  batch_id,
  storage_backend,
  object_key,
  content_type,
  byte_count,
  sha256_hex,
  event_count,
  metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (batch_id, object_key) DO UPDATE SET
  metadata = EXCLUDED.metadata
WHERE api_usage_export_artifacts.sha256_hex = EXCLUDED.sha256_hex
RETURNING id, batch_id, created_at, storage_backend, object_key, content_type,
  byte_count, sha256_hex, event_count, metadata`
	var artifact APIUsageExportArtifactView
	if err := tx.QueryRowContext(
		ctx,
		query,
		id,
		req.BatchID,
		req.StorageBackend,
		req.ObjectKey,
		req.ContentType,
		req.ByteCount,
		req.SHA256Hex,
		req.EventCount,
		req.Metadata,
	).Scan(
		&artifact.ID,
		&artifact.BatchID,
		&artifact.CreatedAt,
		&artifact.StorageBackend,
		&artifact.ObjectKey,
		&artifact.ContentType,
		&artifact.ByteCount,
		&artifact.SHA256Hex,
		&artifact.EventCount,
		&artifact.Metadata,
	); err != nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("create api usage export artifact: %w", err)
	}
	detail, err := apiUsageExportArtifactAuditDetail(artifact)
	if err != nil {
		return APIUsageExportArtifactView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.artifact_create",
		TargetType: "api_usage_export_artifact",
		TargetID:   artifact.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("record api usage export artifact audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("commit api usage export artifact transaction: %w", err)
	}
	return artifact, nil
}

func apiUsageExportArtifactAuditDetail(artifact APIUsageExportArtifactView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"artifact_id":     artifact.ID,
		"batch_id":        artifact.BatchID,
		"storage_backend": artifact.StorageBackend,
		"object_key":      artifact.ObjectKey,
		"content_type":    artifact.ContentType,
		"byte_count":      artifact.ByteCount,
		"sha256_hex":      artifact.SHA256Hex,
		"event_count":     artifact.EventCount,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export artifact audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageExportArtifacts(ctx context.Context, batchID string, limit int) ([]APIUsageExportArtifactView, error) {
	return r.listAPIUsageExportArtifacts(ctx, batchID, limit, false)
}

func (r *Repository) ListAllAPIUsageExportArtifacts(ctx context.Context, batchID string) ([]APIUsageExportArtifactView, error) {
	return r.listAllAPIUsageExportArtifacts(ctx, batchID)
}

func (r *Repository) listAllAPIUsageExportArtifacts(ctx context.Context, batchID string) ([]APIUsageExportArtifactView, error) {
	return r.listAPIUsageExportArtifacts(ctx, batchID, 0, true)
}

func (r *Repository) listAPIUsageExportArtifacts(ctx context.Context, batchID string, limit int, unbounded bool) ([]APIUsageExportArtifactView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, fmt.Errorf("batch_id is required")
	}
	query := apiUsageExportArtifactsQuery(unbounded)
	args := []any{batchID}
	if !unbounded {
		limit = normalizeLimit(limit)
		args = append(args, limit)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api usage export artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []APIUsageExportArtifactView
	for rows.Next() {
		var artifact APIUsageExportArtifactView
		if err := rows.Scan(
			&artifact.ID,
			&artifact.BatchID,
			&artifact.CreatedAt,
			&artifact.StorageBackend,
			&artifact.ObjectKey,
			&artifact.ContentType,
			&artifact.ByteCount,
			&artifact.SHA256Hex,
			&artifact.EventCount,
			&artifact.Metadata,
		); err != nil {
			return nil, fmt.Errorf("scan api usage export artifact: %w", err)
		}
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export artifacts: %w", err)
	}
	return artifacts, nil
}

func apiUsageExportArtifactsQuery(unbounded bool) string {
	query := `
SELECT id, batch_id, created_at, storage_backend, object_key, content_type,
  byte_count, sha256_hex, event_count, metadata
FROM api_usage_export_artifacts
WHERE batch_id = $1
ORDER BY created_at DESC, id DESC
`
	if !unbounded {
		query += `LIMIT $2`
	}
	return query
}

func (r *Repository) GetAPIUsageExportArtifact(ctx context.Context, batchID string, artifactID string) (APIUsageExportArtifactView, error) {
	if r.db == nil {
		return APIUsageExportArtifactView{}, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	artifactID = strings.TrimSpace(artifactID)
	if batchID == "" {
		return APIUsageExportArtifactView{}, fmt.Errorf("batch_id is required")
	}
	if artifactID == "" {
		return APIUsageExportArtifactView{}, fmt.Errorf("artifact_id is required")
	}
	const query = `
SELECT id, batch_id, created_at, storage_backend, object_key, content_type,
  byte_count, sha256_hex, event_count, metadata
FROM api_usage_export_artifacts
WHERE batch_id = $1
  AND id = $2`
	var artifact APIUsageExportArtifactView
	if err := r.db.QueryRowContext(ctx, query, batchID, artifactID).Scan(
		&artifact.ID,
		&artifact.BatchID,
		&artifact.CreatedAt,
		&artifact.StorageBackend,
		&artifact.ObjectKey,
		&artifact.ContentType,
		&artifact.ByteCount,
		&artifact.SHA256Hex,
		&artifact.EventCount,
		&artifact.Metadata,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportArtifactView{}, fmt.Errorf("api usage export artifact not found")
		}
		return APIUsageExportArtifactView{}, fmt.Errorf("get api usage export artifact: %w", err)
	}
	return artifact, nil
}

func ValidateCreateAPIUsageExportArtifactRequest(req *CreateAPIUsageExportArtifactRequest) error {
	req.BatchID = strings.TrimSpace(req.BatchID)
	req.StorageBackend = strings.TrimSpace(req.StorageBackend)
	if strings.ContainsAny(req.ObjectKey, "\r\n") {
		return fmt.Errorf("object_key cannot contain line breaks")
	}
	req.ObjectKey = strings.TrimSpace(req.ObjectKey)
	req.ContentType = strings.TrimSpace(req.ContentType)
	req.SHA256Hex = strings.ToLower(strings.TrimSpace(req.SHA256Hex))
	if req.BatchID == "" {
		return fmt.Errorf("batch_id is required")
	}
	if req.StorageBackend == "" {
		req.StorageBackend = "external"
	}
	if req.ObjectKey == "" {
		return fmt.Errorf("object_key is required")
	}
	if req.ContentType == "" {
		req.ContentType = "application/x-ndjson"
	}
	if req.ContentType != "application/x-ndjson" {
		return fmt.Errorf("content_type must be application/x-ndjson")
	}
	if req.ByteCount < 0 {
		return fmt.Errorf("byte_count must be nonnegative")
	}
	if req.EventCount < 0 {
		return fmt.Errorf("event_count must be nonnegative")
	}
	if !isLowerHexSHA256(req.SHA256Hex) {
		return fmt.Errorf("sha256_hex must be 64 lowercase hex characters")
	}
	if len(req.Metadata) == 0 {
		req.Metadata = json.RawMessage(`{}`)
	}
	var metadata map[string]any
	if err := json.Unmarshal(req.Metadata, &metadata); err != nil {
		return fmt.Errorf("metadata must be a JSON object: %w", err)
	}
	return nil
}

func isLowerHexSHA256(value string) bool {
	return isLowerHexBytes(value, 32)
}

func isLowerHexBytes(value string, bytes int) bool {
	if len(value) != bytes*2 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func newAPIUsageExportArtifactID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export artifact id: %w", err)
	}
	return "api-usage-artifact-" + hex.EncodeToString(random[:]), nil
}
