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
