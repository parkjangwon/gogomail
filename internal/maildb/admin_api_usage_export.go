package maildb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/audit"
)

func (r *Repository) CreateAPIUsageExportBatch(ctx context.Context, req APIUsageLedgerListRequest) (APIUsageExportBatchView, error) {
	if r.db == nil {
		return APIUsageExportBatchView{}, fmt.Errorf("database handle is required")
	}
	stats, err := r.GetAPIUsageLedgerStats(ctx, req)
	if err != nil {
		return APIUsageExportBatchView{}, err
	}
	id, err := newAPIUsageExportBatchID()
	if err != nil {
		return APIUsageExportBatchView{}, err
	}
	manifest, err := json.Marshal(map[string]any{
		"version":      "2026-05-04.api-usage-export.v1",
		"tenant_id":    strings.TrimSpace(req.TenantID),
		"principal_id": strings.TrimSpace(req.PrincipalID),
		"from":         optionalTimeString(req.From),
		"to":           optionalTimeString(req.To),
		"format":       "ndjson",
	})
	if err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("marshal api usage export manifest: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("begin api usage export batch transaction: %w", err)
	}
	defer tx.Rollback()

	var batch APIUsageExportBatchView
	var completedAt sql.NullTime
	var windowStart sql.NullTime
	var windowEnd sql.NullTime
	var firstEventAt sql.NullTime
	var lastEventAt sql.NullTime
	const query = `
INSERT INTO api_usage_export_batches (
  id,
  completed_at,
  status,
  export_format,
  tenant_id,
  principal_id,
  window_start,
  window_end,
  event_count,
  request_count,
  request_bytes,
  response_bytes,
  latency_ms_total,
  latency_ms_max,
  first_event_at,
  last_event_at,
  manifest
) VALUES ($1, now(), 'completed', 'ndjson', $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id, created_at, completed_at, status, export_format, tenant_id, principal_id,
  window_start, window_end, event_count, request_count, request_bytes, response_bytes,
  latency_ms_total, latency_ms_max, first_event_at, last_event_at, manifest`
	if err := tx.QueryRowContext(
		ctx,
		query,
		id,
		strings.TrimSpace(req.TenantID),
		strings.TrimSpace(req.PrincipalID),
		nullableTime(req.From),
		nullableTime(req.To),
		stats.EventCount,
		stats.RequestCount,
		stats.RequestBytes,
		stats.ResponseBytes,
		stats.LatencyMSTotal,
		stats.LatencyMSMax,
		nullableTimePtr(stats.FirstEventAt),
		nullableTimePtr(stats.LastEventAt),
		manifest,
	).Scan(
		&batch.ID,
		&batch.CreatedAt,
		&completedAt,
		&batch.Status,
		&batch.ExportFormat,
		&batch.TenantID,
		&batch.PrincipalID,
		&windowStart,
		&windowEnd,
		&batch.EventCount,
		&batch.RequestCount,
		&batch.RequestBytes,
		&batch.ResponseBytes,
		&batch.LatencyMSTotal,
		&batch.LatencyMSMax,
		&firstEventAt,
		&lastEventAt,
		&batch.Manifest,
	); err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("create api usage export batch: %w", err)
	}
	applyExportBatchNullableTimes(&batch, completedAt, windowStart, windowEnd, firstEventAt, lastEventAt)
	detail, err := apiUsageExportBatchAuditDetail(batch)
	if err != nil {
		return APIUsageExportBatchView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.batch_create",
		TargetType: "api_usage_export_batch",
		TargetID:   batch.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("record api usage export batch audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportBatchView{}, fmt.Errorf("commit api usage export batch transaction: %w", err)
	}
	return batch, nil
}

func apiUsageExportBatchAuditDetail(batch APIUsageExportBatchView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"batch_id":         batch.ID,
		"tenant_id":        batch.TenantID,
		"principal_id":     batch.PrincipalID,
		"status":           batch.Status,
		"export_format":    batch.ExportFormat,
		"window_start":     optionalTimeStringPtr(batch.WindowStart),
		"window_end":       optionalTimeStringPtr(batch.WindowEnd),
		"event_count":      batch.EventCount,
		"request_count":    batch.RequestCount,
		"request_bytes":    batch.RequestBytes,
		"response_bytes":   batch.ResponseBytes,
		"latency_ms_total": batch.LatencyMSTotal,
		"latency_ms_max":   batch.LatencyMSMax,
		"first_event_at":   optionalTimeStringPtr(batch.FirstEventAt),
		"last_event_at":    optionalTimeStringPtr(batch.LastEventAt),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export batch audit detail: %w", err)
	}
	return detail, nil
}

func ValidateAPIUsageExportBatchListRequest(req APIUsageExportBatchListRequest) error {
	for field, value := range map[string]string{
		"tenant_id":    strings.TrimSpace(req.TenantID),
		"principal_id": strings.TrimSpace(req.PrincipalID),
	} {
		if err := validatePushNotificationFilter(field, value); err != nil {
			return err
		}
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "" && !isAPIUsageExportBatchStatus(status) {
		return fmt.Errorf("unsupported api usage export batch status %q", req.Status)
	}
	if !req.From.IsZero() && !req.To.IsZero() && !req.From.Before(req.To) {
		return fmt.Errorf("from must be before to")
	}
	return nil
}

func isAPIUsageExportBatchStatus(status string) bool {
	switch status {
	case "pending", "completed", "failed":
		return true
	default:
		return false
	}
}

func buildListAPIUsageExportBatchesQuery(req APIUsageExportBatchListRequest) (string, []any) {
	query := listAPIUsageExportBatchesBaseSQL
	var conditions []string
	var args []any

	tenantID := strings.TrimSpace(req.TenantID)
	if tenantID != "" {
		args = append(args, tenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	principalID := strings.TrimSpace(req.PrincipalID)
	if principalID != "" {
		args = append(args, principalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "" {
		args = append(args, status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if !req.From.IsZero() {
		args = append(args, req.From.UTC())
		conditions = append(conditions, fmt.Sprintf("window_start >= $%d", len(args)))
	}
	if !req.To.IsZero() {
		args = append(args, req.To.UTC())
		conditions = append(conditions, fmt.Sprintf("window_end < $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	args = append(args, normalizeLimit(req.Limit))
	query += fmt.Sprintf(`
ORDER BY created_at DESC, id DESC
LIMIT $%d`, len(args))
	return query, args
}

func (r *Repository) ListAPIUsageExportBatches(ctx context.Context, req APIUsageExportBatchListRequest) ([]APIUsageExportBatchView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if err := ValidateAPIUsageExportBatchListRequest(req); err != nil {
		return nil, err
	}
	query, args := buildListAPIUsageExportBatchesQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api usage export batches: %w", err)
	}
	defer rows.Close()

	var batches []APIUsageExportBatchView
	for rows.Next() {
		var batch APIUsageExportBatchView
		var completedAt sql.NullTime
		var windowStart sql.NullTime
		var windowEnd sql.NullTime
		var firstEventAt sql.NullTime
		var lastEventAt sql.NullTime
		if err := rows.Scan(
			&batch.ID,
			&batch.CreatedAt,
			&completedAt,
			&batch.Status,
			&batch.ExportFormat,
			&batch.TenantID,
			&batch.PrincipalID,
			&windowStart,
			&windowEnd,
			&batch.EventCount,
			&batch.RequestCount,
			&batch.RequestBytes,
			&batch.ResponseBytes,
			&batch.LatencyMSTotal,
			&batch.LatencyMSMax,
			&firstEventAt,
			&lastEventAt,
			&batch.Manifest,
		); err != nil {
			return nil, fmt.Errorf("scan api usage export batch: %w", err)
		}
		applyExportBatchNullableTimes(&batch, completedAt, windowStart, windowEnd, firstEventAt, lastEventAt)
		batches = append(batches, batch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export batches: %w", err)
	}
	return batches, nil
}

func (r *Repository) GetAPIUsageExportBatch(ctx context.Context, id string) (APIUsageExportBatchView, error) {
	if r.db == nil {
		return APIUsageExportBatchView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return APIUsageExportBatchView{}, fmt.Errorf("api usage export batch id is required")
	}
	const query = `
SELECT id, created_at, completed_at, status, export_format, tenant_id, principal_id,
  window_start, window_end, event_count, request_count, request_bytes, response_bytes,
  latency_ms_total, latency_ms_max, first_event_at, last_event_at, manifest
FROM api_usage_export_batches
WHERE id = $1`
	var batch APIUsageExportBatchView
	var completedAt sql.NullTime
	var windowStart sql.NullTime
	var windowEnd sql.NullTime
	var firstEventAt sql.NullTime
	var lastEventAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&batch.ID,
		&batch.CreatedAt,
		&completedAt,
		&batch.Status,
		&batch.ExportFormat,
		&batch.TenantID,
		&batch.PrincipalID,
		&windowStart,
		&windowEnd,
		&batch.EventCount,
		&batch.RequestCount,
		&batch.RequestBytes,
		&batch.ResponseBytes,
		&batch.LatencyMSTotal,
		&batch.LatencyMSMax,
		&firstEventAt,
		&lastEventAt,
		&batch.Manifest,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportBatchView{}, fmt.Errorf("api usage export batch not found")
		}
		return APIUsageExportBatchView{}, fmt.Errorf("get api usage export batch: %w", err)
	}
	applyExportBatchNullableTimes(&batch, completedAt, windowStart, windowEnd, firstEventAt, lastEventAt)
	return batch, nil
}

func (r *Repository) GetAPIUsageExportHandoff(ctx context.Context, batchID string) (APIUsageExportHandoffView, error) {
	if r.db == nil {
		return APIUsageExportHandoffView{}, fmt.Errorf("database handle is required")
	}
	batch, err := r.GetAPIUsageExportBatch(ctx, batchID)
	if err != nil {
		return APIUsageExportHandoffView{}, err
	}
	view := APIUsageExportHandoffView{
		BatchID:        batch.ID,
		BatchStatus:    batch.Status,
		BatchCompleted: batch.Status == "completed" && batch.CompletedAt != nil,
		EventCount:     batch.EventCount,
	}
	const artifactQuery = `
SELECT count(*), coalesce(sum(event_count), 0), coalesce(sum(byte_count), 0)
FROM api_usage_export_artifacts
WHERE batch_id = $1`
	if err := r.db.QueryRowContext(ctx, artifactQuery, batch.ID).Scan(
		&view.ArtifactCount,
		&view.ArtifactEventCount,
		&view.ArtifactByteCount,
	); err != nil {
		return APIUsageExportHandoffView{}, fmt.Errorf("get api usage export artifact handoff stats: %w", err)
	}

	var latestDigestAt sql.NullTime
	const digestQuery = `
SELECT count(*),
  coalesce((array_agg(id ORDER BY created_at DESC, id DESC))[1], ''),
  coalesce((array_agg(digest_hex ORDER BY created_at DESC, id DESC))[1], ''),
  (array_agg(created_at ORDER BY created_at DESC, id DESC))[1]
FROM api_usage_export_manifest_digests
WHERE batch_id = $1`
	if err := r.db.QueryRowContext(ctx, digestQuery, batch.ID).Scan(
		&view.ManifestDigestCount,
		&view.LatestManifestDigestID,
		&view.LatestManifestDigestHex,
		&latestDigestAt,
	); err != nil {
		return APIUsageExportHandoffView{}, fmt.Errorf("get api usage export manifest digest handoff stats: %w", err)
	}
	if latestDigestAt.Valid {
		view.LatestManifestDigestAt = &latestDigestAt.Time
	}

	if view.LatestManifestDigestID != "" {
		var latestSignatureAt sql.NullTime
		const signatureQuery = `
SELECT count(*),
  coalesce((array_agg(id ORDER BY created_at DESC, id DESC))[1], ''),
  coalesce((array_agg(signer_backend ORDER BY created_at DESC, id DESC))[1], ''),
  coalesce((array_agg(key_id ORDER BY created_at DESC, id DESC))[1], ''),
  (array_agg(created_at ORDER BY created_at DESC, id DESC))[1]
FROM api_usage_export_manifest_signatures
WHERE batch_id = $1
  AND digest_id = $2`
		if err := r.db.QueryRowContext(ctx, signatureQuery, batch.ID, view.LatestManifestDigestID).Scan(
			&view.LatestDigestSignatureCount,
			&view.LatestSignatureID,
			&view.LatestSignatureSigner,
			&view.LatestSignatureKeyID,
			&latestSignatureAt,
		); err != nil {
			return APIUsageExportHandoffView{}, fmt.Errorf("get api usage export manifest signature handoff stats: %w", err)
		}
		if latestSignatureAt.Valid {
			view.LatestSignatureAt = &latestSignatureAt.Time
		}
	}

	applyAPIUsageExportHandoffReadiness(&view)
	return view, nil
}

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

func (r *Repository) CreateAPIUsageExportManifestDigest(ctx context.Context, batchID string) (APIUsageExportManifestDigestView, error) {
	if r.db == nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("database handle is required")
	}
	batch, err := r.GetAPIUsageExportBatch(ctx, batchID)
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	artifacts, err := r.listAllAPIUsageExportArtifacts(ctx, batch.ID)
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	manifest := apiUsageExportManifest(batch, artifacts)
	digest, raw, err := apimeter.DigestExportManifest(manifest)
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	id, err := newAPIUsageExportManifestDigestID()
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("begin api usage export manifest digest transaction: %w", err)
	}
	defer tx.Rollback()
	const query = `
INSERT INTO api_usage_export_manifest_digests (
  id,
  batch_id,
  schema_version,
  digest_algorithm,
  digest_hex,
  manifest
) VALUES ($1, $2, $3, 'sha256', $4, $5)
ON CONFLICT (batch_id, digest_algorithm, digest_hex) DO UPDATE SET
  manifest = EXCLUDED.manifest
RETURNING id, batch_id, created_at, schema_version, digest_algorithm, digest_hex, manifest`
	var view APIUsageExportManifestDigestView
	if err := tx.QueryRowContext(ctx, query, id, batch.ID, manifest.SchemaVersion, digest, raw).Scan(
		&view.ID,
		&view.BatchID,
		&view.CreatedAt,
		&view.SchemaVersion,
		&view.DigestAlgorithm,
		&view.DigestHex,
		&view.Manifest,
	); err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("create api usage export manifest digest: %w", err)
	}
	detail, err := apiUsageExportManifestDigestAuditDetail(view, len(manifest.Artifacts))
	if err != nil {
		return APIUsageExportManifestDigestView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.manifest_digest_create",
		TargetType: "api_usage_export_manifest_digest",
		TargetID:   view.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("record api usage export manifest digest audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("commit api usage export manifest digest transaction: %w", err)
	}
	return view, nil
}

func apiUsageExportManifestDigestAuditDetail(digest APIUsageExportManifestDigestView, artifactCount int) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"digest_id":        digest.ID,
		"batch_id":         digest.BatchID,
		"schema_version":   digest.SchemaVersion,
		"digest_algorithm": digest.DigestAlgorithm,
		"digest_hex":       digest.DigestHex,
		"manifest_bytes":   len(digest.Manifest),
		"artifact_count":   artifactCount,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export manifest digest audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageExportManifestDigests(ctx context.Context, batchID string, limit int) ([]APIUsageExportManifestDigestView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return nil, fmt.Errorf("batch_id is required")
	}
	limit = normalizeLimit(limit)
	const query = `
SELECT id, batch_id, created_at, schema_version, digest_algorithm, digest_hex, manifest
FROM api_usage_export_manifest_digests
WHERE batch_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2`
	rows, err := r.db.QueryContext(ctx, query, batchID, limit)
	if err != nil {
		return nil, fmt.Errorf("list api usage export manifest digests: %w", err)
	}
	defer rows.Close()

	var digests []APIUsageExportManifestDigestView
	for rows.Next() {
		var digest APIUsageExportManifestDigestView
		if err := rows.Scan(
			&digest.ID,
			&digest.BatchID,
			&digest.CreatedAt,
			&digest.SchemaVersion,
			&digest.DigestAlgorithm,
			&digest.DigestHex,
			&digest.Manifest,
		); err != nil {
			return nil, fmt.Errorf("scan api usage export manifest digest: %w", err)
		}
		digests = append(digests, digest)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export manifest digests: %w", err)
	}
	return digests, nil
}

func (r *Repository) GetAPIUsageExportManifestDigest(ctx context.Context, batchID string, digestID string) (APIUsageExportManifestDigestView, error) {
	if r.db == nil {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	digestID = strings.TrimSpace(digestID)
	if batchID == "" {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("batch_id is required")
	}
	if digestID == "" {
		return APIUsageExportManifestDigestView{}, fmt.Errorf("digest_id is required")
	}
	const query = `
SELECT id, batch_id, created_at, schema_version, digest_algorithm, digest_hex, manifest
FROM api_usage_export_manifest_digests
WHERE batch_id = $1
  AND id = $2`
	var digest APIUsageExportManifestDigestView
	if err := r.db.QueryRowContext(ctx, query, batchID, digestID).Scan(
		&digest.ID,
		&digest.BatchID,
		&digest.CreatedAt,
		&digest.SchemaVersion,
		&digest.DigestAlgorithm,
		&digest.DigestHex,
		&digest.Manifest,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportManifestDigestView{}, fmt.Errorf("api usage export manifest digest not found")
		}
		return APIUsageExportManifestDigestView{}, fmt.Errorf("get api usage export manifest digest: %w", err)
	}
	return digest, nil
}

func (r *Repository) VerifyAPIUsageExportManifestDigest(ctx context.Context, batchID string, digestID string) (APIUsageExportManifestDigestVerificationView, error) {
	digest, err := r.GetAPIUsageExportManifestDigest(ctx, batchID, digestID)
	if err != nil {
		return APIUsageExportManifestDigestVerificationView{}, err
	}
	return apiUsageExportManifestDigestVerification(digest)
}

func apiUsageExportManifestDigestVerification(digest APIUsageExportManifestDigestView) (APIUsageExportManifestDigestVerificationView, error) {
	actual, canonical, err := apimeter.DigestExportManifestJSON(digest.Manifest)
	if err != nil {
		return APIUsageExportManifestDigestVerificationView{}, err
	}
	return APIUsageExportManifestDigestVerificationView{
		BatchID:           digest.BatchID,
		DigestID:          digest.ID,
		SchemaVersion:     digest.SchemaVersion,
		DigestAlgorithm:   digest.DigestAlgorithm,
		ExpectedDigestHex: digest.DigestHex,
		ActualDigestHex:   actual,
		Valid:             digest.DigestAlgorithm == "sha256" && digest.DigestHex == actual,
		CanonicalManifest: canonical,
	}, nil
}

func applyAPIUsageExportHandoffReadiness(view *APIUsageExportHandoffView) {
	view.EventsCovered = view.ArtifactEventCount >= view.EventCount
	var missing []string
	if !view.BatchCompleted {
		missing = append(missing, "batch_completed")
	}
	if view.ArtifactCount == 0 {
		missing = append(missing, "export_artifact")
	}
	if !view.EventsCovered {
		missing = append(missing, "event_coverage")
	}
	if view.ManifestDigestCount == 0 || view.LatestManifestDigestID == "" {
		missing = append(missing, "manifest_digest")
	}
	if view.LatestDigestSignatureCount == 0 {
		missing = append(missing, "manifest_signature")
	}
	view.MissingRequirements = missing
	view.Ready = len(missing) == 0
	view.ReadinessGrade = "billing_blocked"
	if !view.Ready {
		view.BillingBlockingReasons = []string{"handoff_not_ready"}
		return
	}
	if apiUsageExportManifestSignerNeedsProductionBackend(view.LatestSignatureSigner) {
		view.ReadinessGrade = "operational"
		view.BillingBlockingReasons = []string{"production_manifest_signer_required"}
		return
	}
	view.ReadinessGrade = "billing_candidate"
	view.BillingReady = true
}

func apiUsageExportManifestSignerNeedsProductionBackend(backend string) bool {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "", "local-hmac", "local-ed25519":
		return true
	default:
		return false
	}
}

func (r *Repository) CreateAPIUsageExportManifestSignature(ctx context.Context, req CreateAPIUsageExportManifestSignatureRequest) (APIUsageExportManifestSignatureView, error) {
	if r.db == nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateAPIUsageExportManifestSignatureRequest(&req); err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	digest, err := r.GetAPIUsageExportManifestDigest(ctx, req.BatchID, req.DigestID)
	if err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	if digest.DigestHex != req.Signature.SignedDigestHex {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("signed_digest_hex must match manifest digest")
	}
	id, err := newAPIUsageExportManifestSignatureID()
	if err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("begin api usage export manifest signature transaction: %w", err)
	}
	defer tx.Rollback()
	const query = `
INSERT INTO api_usage_export_manifest_signatures (
  id,
  digest_id,
  batch_id,
  signer_backend,
  key_id,
  signature_algorithm,
  signed_digest_hex,
  signature_hex,
  metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (digest_id, signature_algorithm, key_id, signature_hex) DO UPDATE SET
  metadata = EXCLUDED.metadata
RETURNING id, digest_id, batch_id, created_at, signer_backend, key_id,
  signature_algorithm, signed_digest_hex, signature_hex, metadata`
	var view APIUsageExportManifestSignatureView
	if err := tx.QueryRowContext(
		ctx,
		query,
		id,
		req.DigestID,
		req.BatchID,
		req.SignerBackend,
		req.Signature.KeyID,
		req.Signature.Algorithm,
		req.Signature.SignedDigestHex,
		req.Signature.SignatureHex,
		req.Metadata,
	).Scan(
		&view.ID,
		&view.DigestID,
		&view.BatchID,
		&view.CreatedAt,
		&view.SignerBackend,
		&view.KeyID,
		&view.SignatureAlgorithm,
		&view.SignedDigestHex,
		&view.SignatureHex,
		&view.Metadata,
	); err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("create api usage export manifest signature: %w", err)
	}
	detail, err := apiUsageExportManifestSignatureAuditDetail(view)
	if err != nil {
		return APIUsageExportManifestSignatureView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage_export.manifest_signature_create",
		TargetType: "api_usage_export_manifest_signature",
		TargetID:   view.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("record api usage export manifest signature audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("commit api usage export manifest signature transaction: %w", err)
	}
	return view, nil
}

func apiUsageExportManifestSignatureAuditDetail(signature APIUsageExportManifestSignatureView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"signature_id":        signature.ID,
		"digest_id":           signature.DigestID,
		"batch_id":            signature.BatchID,
		"signer_backend":      signature.SignerBackend,
		"key_id":              signature.KeyID,
		"signature_algorithm": signature.SignatureAlgorithm,
		"signed_digest_hex":   signature.SignedDigestHex,
		"signature_hex_len":   len(signature.SignatureHex),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage export manifest signature audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageExportManifestSignatures(ctx context.Context, batchID string, digestID string, limit int) ([]APIUsageExportManifestSignatureView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	digestID = strings.TrimSpace(digestID)
	if batchID == "" {
		return nil, fmt.Errorf("batch_id is required")
	}
	if digestID == "" {
		return nil, fmt.Errorf("digest_id is required")
	}
	limit = normalizeLimit(limit)
	const query = `
SELECT id, digest_id, batch_id, created_at, signer_backend, key_id,
  signature_algorithm, signed_digest_hex, signature_hex, metadata
FROM api_usage_export_manifest_signatures
WHERE batch_id = $1
  AND digest_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3`
	rows, err := r.db.QueryContext(ctx, query, batchID, digestID, limit)
	if err != nil {
		return nil, fmt.Errorf("list api usage export manifest signatures: %w", err)
	}
	defer rows.Close()

	var signatures []APIUsageExportManifestSignatureView
	for rows.Next() {
		var signature APIUsageExportManifestSignatureView
		if err := scanAPIUsageExportManifestSignature(rows, &signature); err != nil {
			return nil, fmt.Errorf("scan api usage export manifest signature: %w", err)
		}
		signatures = append(signatures, signature)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage export manifest signatures: %w", err)
	}
	return signatures, nil
}

func (r *Repository) GetAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string, signatureID string) (APIUsageExportManifestSignatureView, error) {
	if r.db == nil {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("database handle is required")
	}
	batchID = strings.TrimSpace(batchID)
	digestID = strings.TrimSpace(digestID)
	signatureID = strings.TrimSpace(signatureID)
	if batchID == "" {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("batch_id is required")
	}
	if digestID == "" {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("digest_id is required")
	}
	if signatureID == "" {
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("signature_id is required")
	}
	const query = `
SELECT id, digest_id, batch_id, created_at, signer_backend, key_id,
  signature_algorithm, signed_digest_hex, signature_hex, metadata
FROM api_usage_export_manifest_signatures
WHERE batch_id = $1
  AND digest_id = $2
  AND id = $3`
	var signature APIUsageExportManifestSignatureView
	if err := scanAPIUsageExportManifestSignature(r.db.QueryRowContext(ctx, query, batchID, digestID, signatureID), &signature); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageExportManifestSignatureView{}, fmt.Errorf("api usage export manifest signature not found")
		}
		return APIUsageExportManifestSignatureView{}, fmt.Errorf("get api usage export manifest signature: %w", err)
	}
	return signature, nil
}

type apiUsageExportManifestSignatureScanner interface {
	Scan(dest ...any) error
}

func scanAPIUsageExportManifestSignature(scanner apiUsageExportManifestSignatureScanner, signature *APIUsageExportManifestSignatureView) error {
	return scanner.Scan(
		&signature.ID,
		&signature.DigestID,
		&signature.BatchID,
		&signature.CreatedAt,
		&signature.SignerBackend,
		&signature.KeyID,
		&signature.SignatureAlgorithm,
		&signature.SignedDigestHex,
		&signature.SignatureHex,
		&signature.Metadata,
	)
}

func ValidateCreateAPIUsageExportManifestSignatureRequest(req *CreateAPIUsageExportManifestSignatureRequest) error {
	req.BatchID = strings.TrimSpace(req.BatchID)
	req.DigestID = strings.TrimSpace(req.DigestID)
	req.SignerBackend = strings.TrimSpace(req.SignerBackend)
	req.Signature.Algorithm = strings.TrimSpace(req.Signature.Algorithm)
	req.Signature.KeyID = strings.TrimSpace(req.Signature.KeyID)
	req.Signature.SignedDigestHex = strings.ToLower(strings.TrimSpace(req.Signature.SignedDigestHex))
	req.Signature.SignatureHex = strings.ToLower(strings.TrimSpace(req.Signature.SignatureHex))
	if req.BatchID == "" {
		return fmt.Errorf("batch_id is required")
	}
	if req.DigestID == "" {
		return fmt.Errorf("digest_id is required")
	}
	if req.SignerBackend == "" {
		return fmt.Errorf("signer_backend is required")
	}
	switch req.Signature.Algorithm {
	case apimeter.ExportManifestSignatureAlgorithmHMACSHA256, apimeter.ExportManifestSignatureAlgorithmEd25519:
	default:
		return fmt.Errorf("signature_algorithm must be hmac-sha256 or ed25519")
	}
	if !apiUsageExportManifestSignatureBackendMatchesAlgorithm(req.SignerBackend, req.Signature.Algorithm) {
		return fmt.Errorf("signer_backend %q is not compatible with signature_algorithm %q", req.SignerBackend, req.Signature.Algorithm)
	}
	if req.Signature.KeyID == "" {
		return fmt.Errorf("key_id is required")
	}
	if !isLowerHexSHA256(req.Signature.SignedDigestHex) {
		return fmt.Errorf("signed_digest_hex must be 64 lowercase hex characters")
	}
	if req.Signature.Algorithm == apimeter.ExportManifestSignatureAlgorithmHMACSHA256 && !isLowerHexBytes(req.Signature.SignatureHex, 32) {
		return fmt.Errorf("signature_hex must be 64 lowercase hex characters")
	}
	if req.Signature.Algorithm == apimeter.ExportManifestSignatureAlgorithmEd25519 && !isLowerHexBytes(req.Signature.SignatureHex, 64) {
		return fmt.Errorf("signature_hex must be 128 lowercase hex characters")
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

func apiUsageExportManifestSignatureBackendMatchesAlgorithm(backend string, algorithm string) bool {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "local-hmac":
		return algorithm == apimeter.ExportManifestSignatureAlgorithmHMACSHA256
	case "local-ed25519":
		return algorithm == apimeter.ExportManifestSignatureAlgorithmEd25519
	default:
		return true
	}
}

func apiUsageExportManifest(batch APIUsageExportBatchView, artifacts []APIUsageExportArtifactView) apimeter.ExportManifest {
	ordered := append([]APIUsageExportArtifactView(nil), artifacts...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].ID < ordered[j].ID
	})
	manifest := apimeter.ExportManifest{
		SchemaVersion: apimeter.ExportManifestSchemaV1,
		Batch: apimeter.ExportManifestBatch{
			ID:             batch.ID,
			TenantID:       batch.TenantID,
			PrincipalID:    batch.PrincipalID,
			WindowStart:    apimeter.FormatManifestTime(batch.WindowStart),
			WindowEnd:      apimeter.FormatManifestTime(batch.WindowEnd),
			EventCount:     batch.EventCount,
			RequestCount:   batch.RequestCount,
			RequestBytes:   batch.RequestBytes,
			ResponseBytes:  batch.ResponseBytes,
			LatencyMSTotal: batch.LatencyMSTotal,
			LatencyMSMax:   batch.LatencyMSMax,
		},
		Artifacts: make([]apimeter.ExportManifestArtifact, 0, len(ordered)),
	}
	for _, artifact := range ordered {
		manifest.Artifacts = append(manifest.Artifacts, apimeter.ExportManifestArtifact{
			ID:             artifact.ID,
			StorageBackend: artifact.StorageBackend,
			ObjectKey:      artifact.ObjectKey,
			ContentType:    artifact.ContentType,
			ByteCount:      artifact.ByteCount,
			SHA256Hex:      artifact.SHA256Hex,
			EventCount:     artifact.EventCount,
		})
	}
	return manifest
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

func newAPIUsageExportBatchID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export batch id: %w", err)
	}
	return "api-usage-export-" + hex.EncodeToString(random[:]), nil
}

func newAPIUsageLedgerRetentionRunID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage ledger retention run id: %w", err)
	}
	return "api-usage-retention-" + hex.EncodeToString(random[:]), nil
}

func newAPIUsageExportArtifactID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export artifact id: %w", err)
	}
	return "api-usage-artifact-" + hex.EncodeToString(random[:]), nil
}

func newAPIUsageExportManifestDigestID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export manifest digest id: %w", err)
	}
	return "api-usage-manifest-" + hex.EncodeToString(random[:]), nil
}

func newAPIUsageExportManifestSignatureID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate api usage export manifest signature id: %w", err)
	}
	return "api-usage-signature-" + hex.EncodeToString(random[:]), nil
}
