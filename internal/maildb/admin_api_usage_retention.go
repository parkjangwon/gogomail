package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
)

func (r *Repository) GetAPIUsageLedgerRetentionReadiness(ctx context.Context, req APIUsageLedgerRetentionRequest) (APIUsageLedgerRetentionReadinessView, error) {
	if r.db == nil {
		return APIUsageLedgerRetentionReadinessView{}, fmt.Errorf("database handle is required")
	}
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.PrincipalID = strings.TrimSpace(req.PrincipalID)
	if req.Cutoff.IsZero() {
		return APIUsageLedgerRetentionReadinessView{}, fmt.Errorf("cutoff is required")
	}
	req.Cutoff = req.Cutoff.UTC()
	if req.Cutoff.After(time.Now().UTC()) {
		return APIUsageLedgerRetentionReadinessView{}, fmt.Errorf("cutoff must not be in the future")
	}

	view := APIUsageLedgerRetentionReadinessView{
		Cutoff:      req.Cutoff,
		TenantID:    req.TenantID,
		PrincipalID: req.PrincipalID,
	}
	query := `
SELECT
  count(*)::bigint,
  COALESCE(sum(request_count), 0)::bigint,
  COALESCE(sum(request_bytes), 0)::bigint,
  COALESCE(sum(response_bytes), 0)::bigint,
  COALESCE(sum(latency_ms), 0)::bigint,
  COALESCE(max(latency_ms), 0)::bigint,
  min(event_timestamp),
  max(event_timestamp),
  max(recorded_at)
FROM api_usage_ledger`
	var conditions []string
	var args []any
	args = append(args, req.Cutoff)
	conditions = append(conditions, fmt.Sprintf("event_timestamp < $%d", len(args)))
	if req.TenantID != "" {
		args = append(args, req.TenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if req.PrincipalID != "" {
		args = append(args, req.PrincipalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	query += "\nWHERE " + strings.Join(conditions, "\n  AND ")

	var firstCandidateAt sql.NullTime
	var lastCandidateAt sql.NullTime
	var latestRecordedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&view.CandidateEventCount,
		&view.CandidateRequestCount,
		&view.CandidateRequestBytes,
		&view.CandidateResponseBytes,
		&view.CandidateLatencyMSTotal,
		&view.CandidateLatencyMSMax,
		&firstCandidateAt,
		&lastCandidateAt,
		&latestRecordedAt,
	); err != nil {
		return APIUsageLedgerRetentionReadinessView{}, fmt.Errorf("get api usage ledger retention candidates: %w", err)
	}
	if firstCandidateAt.Valid {
		view.FirstCandidateEventAt = &firstCandidateAt.Time
	}
	if lastCandidateAt.Valid {
		view.LastCandidateEventAt = &lastCandidateAt.Time
	}
	if latestRecordedAt.Valid {
		view.LatestCandidateRecordedAt = &latestRecordedAt.Time
	}
	if view.CandidateEventCount > 0 && view.FirstCandidateEventAt != nil {
		if err := r.findAPIUsageLedgerRetentionCoveringBatch(ctx, req, view.FirstCandidateEventAt, &view); err != nil {
			return APIUsageLedgerRetentionReadinessView{}, err
		}
	}
	applyAPIUsageLedgerRetentionReadiness(&view)
	return view, nil
}

func NormalizeAPIUsageLedgerRetentionLimit(limit int) int {
	if limit <= 0 {
		return APIUsageLedgerRetentionDefaultLimit
	}
	if limit > APIUsageLedgerRetentionMaxLimit {
		return APIUsageLedgerRetentionMaxLimit
	}
	return limit
}

func (r *Repository) RunAPIUsageLedgerRetention(ctx context.Context, req APIUsageLedgerRetentionRunRequest) (APIUsageLedgerRetentionRunView, error) {
	if r.db == nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("database handle is required")
	}
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.PrincipalID = strings.TrimSpace(req.PrincipalID)
	if req.Cutoff.IsZero() {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("cutoff is required")
	}
	req.Cutoff = req.Cutoff.UTC()
	if req.Cutoff.After(time.Now().UTC()) {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("cutoff must not be in the future")
	}
	if req.Limit < 0 {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("limit must not be negative")
	}
	if !req.DryRun && !req.ConfirmReady {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("confirm_ready is required for destructive retention runs")
	}
	limit := NormalizeAPIUsageLedgerRetentionLimit(req.Limit)
	id, err := newAPIUsageLedgerRetentionRunID()
	if err != nil {
		return APIUsageLedgerRetentionRunView{}, err
	}

	readiness, err := r.GetAPIUsageLedgerRetentionReadiness(ctx, APIUsageLedgerRetentionRequest{
		Cutoff:      req.Cutoff,
		TenantID:    req.TenantID,
		PrincipalID: req.PrincipalID,
	})
	if err != nil {
		return APIUsageLedgerRetentionRunView{}, err
	}
	limited := readiness.CandidateEventCount
	if limited > int64(limit) {
		limited = int64(limit)
	}
	view := APIUsageLedgerRetentionRunView{
		ID:             id,
		Cutoff:         req.Cutoff,
		TenantID:       req.TenantID,
		PrincipalID:    req.PrincipalID,
		Limit:          limit,
		DryRun:         req.DryRun,
		ConfirmReady:   req.ConfirmReady,
		Ready:          readiness.Ready,
		CandidateCount: readiness.CandidateEventCount,
		LimitedCount:   limited,
		Readiness:      readiness,
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("begin api usage ledger retention transaction: %w", err)
	}
	defer tx.Rollback()

	if req.DryRun || !readiness.Ready || limited == 0 {
		if err := r.insertAPIUsageLedgerRetentionRun(ctx, tx, &view); err != nil {
			return APIUsageLedgerRetentionRunView{}, err
		}
		if err := recordAPIUsageLedgerRetentionRunAudit(ctx, tx, view); err != nil {
			return APIUsageLedgerRetentionRunView{}, err
		}
		if err := tx.Commit(); err != nil {
			return APIUsageLedgerRetentionRunView{}, fmt.Errorf("commit api usage ledger retention transaction: %w", err)
		}
		return view, nil
	}

	var conditions []string
	var args []any
	args = append(args, req.Cutoff)
	conditions = append(conditions, fmt.Sprintf("event_timestamp < $%d", len(args)))
	if req.TenantID != "" {
		args = append(args, req.TenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if req.PrincipalID != "" {
		args = append(args, req.PrincipalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	if readiness.CoveringExportBatchCompletedAt != nil {
		args = append(args, readiness.CoveringExportBatchCompletedAt.UTC())
		conditions = append(conditions, fmt.Sprintf("recorded_at <= $%d", len(args)))
	}
	args = append(args, limit)
	limitPlaceholder := fmt.Sprintf("$%d", len(args))
	query := fmt.Sprintf(`
DELETE FROM api_usage_ledger
WHERE event_id IN (
  SELECT event_id
  FROM api_usage_ledger
  WHERE %s
  ORDER BY event_timestamp ASC, event_id ASC
  LIMIT %s
)`, strings.Join(conditions, "\n    AND "), limitPlaceholder)

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("run api usage ledger retention: %w", err)
	}
	view.DeletedCount, _ = result.RowsAffected()
	if err := r.insertAPIUsageLedgerRetentionRun(ctx, tx, &view); err != nil {
		return APIUsageLedgerRetentionRunView{}, err
	}
	if err := recordAPIUsageLedgerRetentionRunAudit(ctx, tx, view); err != nil {
		return APIUsageLedgerRetentionRunView{}, err
	}
	if err := tx.Commit(); err != nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("commit api usage ledger retention transaction: %w", err)
	}
	return view, nil
}

func recordAPIUsageLedgerRetentionRunAudit(ctx context.Context, tx *sql.Tx, view APIUsageLedgerRetentionRunView) error {
	detail, err := apiUsageLedgerRetentionRunAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		Category:   "admin",
		Action:     "api_usage.retention_run",
		TargetType: "api_usage_ledger_retention_run",
		TargetID:   view.ID,
		Result:     apiUsageLedgerRetentionRunAuditResult(view),
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record api usage ledger retention run audit: %w", err)
	}
	return nil
}

func apiUsageLedgerRetentionRunAuditResult(view APIUsageLedgerRetentionRunView) string {
	switch {
	case view.DryRun:
		return "dry_run"
	case !view.Ready:
		return "blocked"
	case view.DeletedCount == 0:
		return "no_op"
	default:
		return "completed"
	}
}

func apiUsageLedgerRetentionRunAuditDetail(view APIUsageLedgerRetentionRunView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"run_id":                             view.ID,
		"cutoff":                             view.Cutoff.UTC().Format(time.RFC3339),
		"tenant_id":                          view.TenantID,
		"principal_id":                       view.PrincipalID,
		"limit":                              view.Limit,
		"dry_run":                            view.DryRun,
		"confirm_ready":                      view.ConfirmReady,
		"ready":                              view.Ready,
		"candidate_count":                    view.CandidateCount,
		"limited_count":                      view.LimitedCount,
		"deleted_count":                      view.DeletedCount,
		"blocking_reasons":                   view.Readiness.BlockingReasons,
		"covering_export_batch_id":           view.Readiness.CoveringExportBatchID,
		"covering_artifact_count":            view.Readiness.CoveringArtifactCount,
		"covering_manifest_digest_count":     view.Readiness.CoveringManifestDigestCount,
		"covering_manifest_signature_count":  view.Readiness.CoveringManifestSignatureCount,
		"covering_export_batch_completed_at": optionalTimeStringPtr(view.Readiness.CoveringExportBatchCompletedAt),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal api usage ledger retention run audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAPIUsageLedgerRetentionRuns(ctx context.Context, req APIUsageLedgerRetentionRunListRequest) ([]APIUsageLedgerRetentionRunView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req.Limit = normalizeLimit(req.Limit)
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.PrincipalID = strings.TrimSpace(req.PrincipalID)

	query := `
SELECT id, created_at, cutoff, tenant_id, principal_id, limit_count, dry_run,
  confirm_ready, ready, candidate_count, limited_count, deleted_count, readiness
FROM api_usage_ledger_retention_runs`
	var conditions []string
	var args []any
	if req.TenantID != "" {
		args = append(args, req.TenantID)
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if req.PrincipalID != "" {
		args = append(args, req.PrincipalID)
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", len(args)))
	}
	if !req.CreatedFrom.IsZero() {
		args = append(args, req.CreatedFrom.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !req.CreatedTo.IsZero() {
		args = append(args, req.CreatedTo.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	args = append(args, req.Limit)
	query += fmt.Sprintf(`
ORDER BY created_at DESC, id DESC
LIMIT $%d`, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api usage ledger retention runs: %w", err)
	}
	defer rows.Close()

	var runs []APIUsageLedgerRetentionRunView
	for rows.Next() {
		run, err := scanAPIUsageLedgerRetentionRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api usage ledger retention runs: %w", err)
	}
	return runs, nil
}

func (r *Repository) GetAPIUsageLedgerRetentionRun(ctx context.Context, id string) (APIUsageLedgerRetentionRunView, error) {
	if r.db == nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("api usage ledger retention run id is required")
	}
	const query = `
SELECT id, created_at, cutoff, tenant_id, principal_id, limit_count, dry_run,
  confirm_ready, ready, candidate_count, limited_count, deleted_count, readiness
FROM api_usage_ledger_retention_runs
WHERE id = $1`
	run, err := scanAPIUsageLedgerRetentionRun(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIUsageLedgerRetentionRunView{}, fmt.Errorf("api usage ledger retention run not found")
		}
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("get api usage ledger retention run: %w", err)
	}
	return run, nil
}

type apiUsageLedgerRetentionRunScanner interface {
	Scan(...any) error
}

func scanAPIUsageLedgerRetentionRun(scanner apiUsageLedgerRetentionRunScanner) (APIUsageLedgerRetentionRunView, error) {
	var run APIUsageLedgerRetentionRunView
	var readiness json.RawMessage
	if err := scanner.Scan(
		&run.ID,
		&run.CreatedAt,
		&run.Cutoff,
		&run.TenantID,
		&run.PrincipalID,
		&run.Limit,
		&run.DryRun,
		&run.ConfirmReady,
		&run.Ready,
		&run.CandidateCount,
		&run.LimitedCount,
		&run.DeletedCount,
		&readiness,
	); err != nil {
		return APIUsageLedgerRetentionRunView{}, fmt.Errorf("scan api usage ledger retention run: %w", err)
	}
	if len(readiness) > 0 {
		if err := json.Unmarshal(readiness, &run.Readiness); err != nil {
			return APIUsageLedgerRetentionRunView{}, fmt.Errorf("decode api usage ledger retention run readiness: %w", err)
		}
	}
	run.CreatedAt = run.CreatedAt.UTC()
	run.Cutoff = run.Cutoff.UTC()
	return run, nil
}

func (r *Repository) insertAPIUsageLedgerRetentionRun(ctx context.Context, execer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, view *APIUsageLedgerRetentionRunView) error {
	readiness, err := json.Marshal(view.Readiness)
	if err != nil {
		return fmt.Errorf("marshal api usage ledger retention readiness: %w", err)
	}
	const query = `
INSERT INTO api_usage_ledger_retention_runs (
  id,
  cutoff,
  tenant_id,
  principal_id,
  limit_count,
  dry_run,
  confirm_ready,
  ready,
  candidate_count,
  limited_count,
  deleted_count,
  readiness
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb)
RETURNING created_at`
	if err := execer.QueryRowContext(
		ctx,
		query,
		view.ID,
		view.Cutoff,
		view.TenantID,
		view.PrincipalID,
		view.Limit,
		view.DryRun,
		view.ConfirmReady,
		view.Ready,
		view.CandidateCount,
		view.LimitedCount,
		view.DeletedCount,
		string(readiness),
	).Scan(&view.CreatedAt); err != nil {
		return fmt.Errorf("record api usage ledger retention run: %w", err)
	}
	view.CreatedAt = view.CreatedAt.UTC()
	return nil
}

func (r *Repository) findAPIUsageLedgerRetentionCoveringBatch(ctx context.Context, req APIUsageLedgerRetentionRequest, firstCandidateAt *time.Time, view *APIUsageLedgerRetentionReadinessView) error {
	var completedAt sql.NullTime
	var windowStart sql.NullTime
	var windowEnd sql.NullTime
	err := r.db.QueryRowContext(ctx, apiUsageLedgerRetentionCoveringBatchSQL, req.TenantID, req.PrincipalID, firstCandidateAt.UTC(), req.Cutoff).Scan(
		&view.CoveringExportBatchID,
		&completedAt,
		&windowStart,
		&windowEnd,
		&view.CoveringExportBatchEventCount,
		&view.CoveringArtifactCount,
		&view.CoveringArtifactEventCount,
		&view.CoveringManifestDigestCount,
		&view.CoveringManifestSignatureCount,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("get api usage ledger retention covering export batch: %w", err)
	}
	if completedAt.Valid {
		view.CoveringExportBatchCompletedAt = &completedAt.Time
	}
	if windowStart.Valid {
		view.CoveringExportBatchWindowStart = &windowStart.Time
	}
	if windowEnd.Valid {
		view.CoveringExportBatchWindowEnd = &windowEnd.Time
	}
	return nil
}

const apiUsageLedgerRetentionCoveringBatchSQL = `
SELECT
  b.id,
  b.completed_at,
  b.window_start,
  b.window_end,
  b.event_count,
  COALESCE(a.artifact_count, 0)::bigint,
  COALESCE(a.artifact_event_count, 0)::bigint,
  COALESCE(d.digest_count, 0)::bigint,
  COALESCE(s.signature_count, 0)::bigint
FROM api_usage_export_batches b
LEFT JOIN LATERAL (
  SELECT count(*)::bigint AS artifact_count, COALESCE(sum(event_count), 0)::bigint AS artifact_event_count
  FROM api_usage_export_artifacts
  WHERE batch_id = b.id
) a ON true
LEFT JOIN LATERAL (
  SELECT count(*)::bigint AS digest_count
  FROM api_usage_export_manifest_digests
  WHERE batch_id = b.id
) d ON true
LEFT JOIN LATERAL (
  SELECT count(*)::bigint AS signature_count
  FROM api_usage_export_manifest_signatures
  WHERE batch_id = b.id
) s ON true
WHERE b.status = 'completed'
  AND b.completed_at IS NOT NULL
  AND b.tenant_id = $1
  AND b.principal_id = $2
  AND COALESCE(b.window_start, '-infinity'::timestamptz) <= $3
  AND b.window_end IS NOT NULL
  AND b.window_end >= $4
ORDER BY b.completed_at DESC, b.id DESC
LIMIT 1`

func applyAPIUsageLedgerRetentionReadiness(view *APIUsageLedgerRetentionReadinessView) {
	var blocking []string
	if view.CandidateEventCount > 0 {
		if view.CoveringExportBatchID == "" {
			blocking = append(blocking, "covering_export_batch_required")
		}
		if view.CoveringExportBatchCompletedAt != nil && view.LatestCandidateRecordedAt != nil && view.CoveringExportBatchCompletedAt.Before(*view.LatestCandidateRecordedAt) {
			blocking = append(blocking, "covering_export_batch_stale")
		}
		if view.CoveringExportBatchID != "" && (view.CoveringArtifactCount == 0 || view.CoveringArtifactEventCount < view.CoveringExportBatchEventCount) {
			blocking = append(blocking, "covering_export_artifact_required")
		}
		if view.CoveringExportBatchID != "" && view.CoveringManifestDigestCount == 0 {
			blocking = append(blocking, "covering_manifest_digest_required")
		}
		if view.CoveringExportBatchID != "" && view.CoveringManifestSignatureCount == 0 {
			blocking = append(blocking, "covering_manifest_signature_required")
		}
	}
	view.BlockingReasons = blocking
	view.Ready = len(blocking) == 0
}
