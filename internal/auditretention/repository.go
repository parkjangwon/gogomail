package auditretention

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Repository persists audit retention policies and purge run records and
// executes the bounded batched purge against audit_logs.
type Repository struct {
	db  *sql.DB
	now func() time.Time
}

// NewRepository constructs a Repository backed by db.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, now: time.Now}
}

// GetPolicy returns the retention policy for companyID, or (nil, nil) when the
// company has no policy (keep forever).
func (r *Repository) GetPolicy(ctx context.Context, companyID string) (*Policy, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, fmt.Errorf("company id is required")
	}
	const query = `
SELECT company_id, retention_days, COALESCE(updated_by::text, ''), updated_at
FROM audit_retention_policies
WHERE company_id = $1::uuid`
	var policy Policy
	if err := r.db.QueryRowContext(ctx, query, companyID).Scan(
		&policy.CompanyID,
		&policy.RetentionDays,
		&policy.UpdatedBy,
		&policy.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get audit retention policy: %w", err)
	}
	policy.UpdatedAt = policy.UpdatedAt.UTC()
	return &policy, nil
}

// SetPolicy upserts a retention policy. retentionDays must be a positive member
// of AllowedRetentionDays; use DeletePolicy to disable retention (keep forever).
func (r *Repository) SetPolicy(ctx context.Context, companyID string, retentionDays int, updatedBy string) (*Policy, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, fmt.Errorf("company id is required")
	}
	if retentionDays == RetentionForever {
		return nil, fmt.Errorf("use DeletePolicy to disable audit retention")
	}
	if err := ValidateRetentionDays(retentionDays); err != nil {
		return nil, err
	}
	updatedBy = strings.TrimSpace(updatedBy)
	const query = `
INSERT INTO audit_retention_policies (company_id, retention_days, updated_by, updated_at)
VALUES ($1::uuid, $2, $3, now())
ON CONFLICT (company_id) DO UPDATE
  SET retention_days = EXCLUDED.retention_days,
      updated_by = EXCLUDED.updated_by,
      updated_at = now()
RETURNING company_id, retention_days, COALESCE(updated_by::text, ''), updated_at`
	var updatedByArg any
	if updatedBy != "" {
		updatedByArg = updatedBy
	}
	var policy Policy
	if err := r.db.QueryRowContext(ctx, query, companyID, retentionDays, updatedByArg).Scan(
		&policy.CompanyID,
		&policy.RetentionDays,
		&policy.UpdatedBy,
		&policy.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("set audit retention policy: %w", err)
	}
	policy.UpdatedAt = policy.UpdatedAt.UTC()
	return &policy, nil
}

// DeletePolicy removes any retention policy for companyID (reverting to keep
// forever). Removing a non-existent policy is a no-op.
func (r *Repository) DeletePolicy(ctx context.Context, companyID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return fmt.Errorf("company id is required")
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM audit_retention_policies WHERE company_id = $1::uuid`, companyID); err != nil {
		return fmt.Errorf("delete audit retention policy: %w", err)
	}
	return nil
}

// ListPolicies returns every stored retention policy, ordered by company id.
// Companies without a policy are absent (they keep logs forever).
func (r *Repository) ListPolicies(ctx context.Context) ([]Policy, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT company_id, retention_days, COALESCE(updated_by::text, ''), updated_at
FROM audit_retention_policies
ORDER BY company_id ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list audit retention policies: %w", err)
	}
	defer rows.Close()
	var policies []Policy
	for rows.Next() {
		var policy Policy
		if err := rows.Scan(&policy.CompanyID, &policy.RetentionDays, &policy.UpdatedBy, &policy.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan audit retention policy: %w", err)
		}
		policy.UpdatedAt = policy.UpdatedAt.UTC()
		policies = append(policies, policy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit retention policies: %w", err)
	}
	return policies, nil
}

// PurgeExpired deletes audit_logs rows for companyID older than cutoff, in
// batches of at most batchSize rows, and returns the total number of rows
// deleted. It never deletes anything when retentionDays/cutoff is not set by a
// caller; callers derive cutoff from a stored policy so a missing policy means
// this is never invoked.
func (r *Repository) PurgeExpired(ctx context.Context, companyID string, cutoff time.Time, batchSize int) (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return 0, fmt.Errorf("company id is required")
	}
	if cutoff.IsZero() {
		return 0, fmt.Errorf("cutoff is required")
	}
	cutoff = cutoff.UTC()
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}
	const query = `
WITH candidates AS (
  SELECT id
  FROM audit_logs
  WHERE company_id = $1::uuid
    AND created_at < $2
  ORDER BY created_at ASC, id ASC
  LIMIT $3
)
DELETE FROM audit_logs a
USING candidates c
WHERE a.id = c.id`
	var total int64
	for {
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}
		res, err := r.db.ExecContext(ctx, query, companyID, cutoff, batchSize)
		if err != nil {
			return total, fmt.Errorf("purge expired audit logs: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, fmt.Errorf("inspect purged audit logs: %w", err)
		}
		total += n
		if n < int64(batchSize) {
			break
		}
	}
	return total, nil
}

// CountExpired reports how many audit_logs rows for companyID predate cutoff.
// It is used by dry-run purge to report what would be deleted without deleting.
func (r *Repository) CountExpired(ctx context.Context, companyID string, cutoff time.Time) (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return 0, fmt.Errorf("company id is required")
	}
	if cutoff.IsZero() {
		return 0, fmt.Errorf("cutoff is required")
	}
	const query = `
SELECT count(*)
FROM audit_logs
WHERE company_id = $1::uuid
  AND created_at < $2`
	var count int64
	if err := r.db.QueryRowContext(ctx, query, companyID, cutoff.UTC()).Scan(&count); err != nil {
		return 0, fmt.Errorf("count expired audit logs: %w", err)
	}
	return count, nil
}

// RecordRun persists a purge run record and returns the normalized record
// (with generated id and timestamps filled in).
func (r *Repository) RecordRun(ctx context.Context, record RunRecord) (RunRecord, error) {
	if r.db == nil {
		return RunRecord{}, fmt.Errorf("database handle is required")
	}
	if r.now == nil {
		r.now = time.Now
	}
	record, err := normalizeRunRecord(record, r.now)
	if err != nil {
		return RunRecord{}, err
	}
	const query = `
INSERT INTO audit_retention_runs (
  id, company_id, retention_days, cutoff, started_at, finished_at, dry_run, status, error_message, rows_deleted
) VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10)`
	if _, err := r.db.ExecContext(ctx, query,
		record.ID,
		record.CompanyID,
		record.RetentionDays,
		record.Cutoff,
		record.StartedAt,
		record.FinishedAt,
		record.DryRun,
		string(record.Status),
		record.ErrorMessage,
		record.RowsDeleted,
	); err != nil {
		return RunRecord{}, fmt.Errorf("record audit retention run: %w", err)
	}
	return record, nil
}

// ListRuns returns recent purge runs for a company, newest first.
func (r *Repository) ListRuns(ctx context.Context, req RunListRequest) ([]RunRecord, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := normalizeRunListRequest(req)
	if err != nil {
		return nil, err
	}
	query := `
SELECT id, company_id, retention_days, cutoff, started_at, finished_at, dry_run, status, error_message, rows_deleted
FROM audit_retention_runs
WHERE company_id = $1::uuid`
	args := []any{req.CompanyID}
	if req.Status != "" {
		args = append(args, string(req.Status))
		query += fmt.Sprintf("\n  AND status = $%d", len(args))
	}
	args = append(args, req.Limit)
	query += fmt.Sprintf("\nORDER BY started_at DESC, id DESC\nLIMIT $%d", len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit retention runs: %w", err)
	}
	defer rows.Close()
	var runs []RunRecord
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit retention runs: %w", err)
	}
	return runs, nil
}

// GetRun returns a single purge run by id.
func (r *Repository) GetRun(ctx context.Context, id string) (RunRecord, error) {
	if r.db == nil {
		return RunRecord{}, fmt.Errorf("database handle is required")
	}
	id, err := validateRunID(id)
	if err != nil {
		return RunRecord{}, err
	}
	const query = `
SELECT id, company_id, retention_days, cutoff, started_at, finished_at, dry_run, status, error_message, rows_deleted
FROM audit_retention_runs
WHERE id = $1`
	run, err := scanRun(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RunRecord{}, fmt.Errorf("audit retention run not found")
		}
		return RunRecord{}, fmt.Errorf("get audit retention run: %w", err)
	}
	return run, nil
}

type runScanner interface {
	Scan(...any) error
}

func scanRun(scanner runScanner) (RunRecord, error) {
	var run RunRecord
	var status string
	if err := scanner.Scan(
		&run.ID,
		&run.CompanyID,
		&run.RetentionDays,
		&run.Cutoff,
		&run.StartedAt,
		&run.FinishedAt,
		&run.DryRun,
		&status,
		&run.ErrorMessage,
		&run.RowsDeleted,
	); err != nil {
		return RunRecord{}, fmt.Errorf("scan audit retention run: %w", err)
	}
	run.Status = RunStatus(status)
	if run.Status != RunStatusCompleted && run.Status != RunStatusFailed {
		return RunRecord{}, fmt.Errorf("scan audit retention run: unsupported status")
	}
	run.Cutoff = run.Cutoff.UTC()
	run.StartedAt = run.StartedAt.UTC()
	run.FinishedAt = run.FinishedAt.UTC()
	return run, nil
}

func newRunID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate audit retention run id: %w", err)
	}
	return "audit-retention-" + hex.EncodeToString(random[:]), nil
}
