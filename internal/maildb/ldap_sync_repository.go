package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateLDAPSyncRun creates a new LDAP sync run record.
func (r *Repository) CreateLDAPSyncRun(ctx context.Context, domainID uuid.UUID, syncType string, strategy *string) (uuid.UUID, error) {
	if r.db == nil {
		return uuid.UUID{}, fmt.Errorf("database handle is required")
	}

	id := uuid.New()
	const query = `
INSERT INTO ldap_sync_runs (id, domain_id, sync_type, status, resolution_strategy, started_at)
VALUES ($1, $2, $3, 'running', $4, now())
RETURNING id`

	var resultID uuid.UUID
	err := r.db.QueryRowContext(ctx, query, id, domainID, syncType, strategy).Scan(&resultID)
	if err != nil {
		return uuid.UUID{}, err
	}
	return resultID, nil
}

// UpdateLDAPSyncRun updates an LDAP sync run with results.
func (r *Repository) UpdateLDAPSyncRun(ctx context.Context, runID uuid.UUID, status string, created, updated, deleted, conflicts, errors int, errorMsg *string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	durationMs := 0
	if status != "running" {
		durationMs = int(time.Since(time.Now()).Milliseconds())
		if durationMs < 0 {
			durationMs = 0
		}
	}

	const query = `
UPDATE ldap_sync_runs
SET status = $2, created_count = $3, updated_count = $4, deleted_count = $5,
    conflict_count = $6, error_count = $7, error_message = $8,
    completed_at = CASE WHEN $2 != 'running' THEN now() ELSE completed_at END,
    last_success_at = CASE WHEN $2 = 'success' THEN now() ELSE last_success_at END,
    duration_ms = $9
WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, runID, status, created, updated, deleted, conflicts, errors, errorMsg, durationMs)
	return err
}

// GetLDAPSyncRuns lists LDAP sync runs for a domain.
func (r *Repository) GetLDAPSyncRuns(ctx context.Context, req LDAPSyncRunListRequest) ([]LDAPSyncRunView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	query := `
SELECT id, domain_id, sync_type, status, created_count, updated_count, deleted_count,
       conflict_count, error_count, resolution_strategy, started_at, completed_at,
       last_success_at, duration_ms, error_message
FROM ldap_sync_runs
WHERE domain_id = $1`

	args := []interface{}{req.DomainID}
	argNum := 2

	if req.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, req.Status)
		argNum++
	}

	query += " ORDER BY started_at DESC"

	if req.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, req.Limit)
		argNum++
	}
	if req.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, req.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []LDAPSyncRunView
	for rows.Next() {
		var run LDAPSyncRunView
		err := rows.Scan(&run.ID, &run.DomainID, &run.SyncType, &run.Status, &run.CreatedCount, &run.UpdatedCount, &run.DeletedCount,
			&run.ConflictCount, &run.ErrorCount, &run.ResolutionStrategy, &run.StartedAt, &run.CompletedAt,
			&run.LastSuccessAt, &run.DurationMs, &run.ErrorMessage)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// AddLDAPSyncConflict records a conflict detected during sync.
func (r *Repository) AddLDAPSyncConflict(ctx context.Context, domainID, runID uuid.UUID, objectType, ldapDN, conflictType string, localValue, ldapValue *string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	const query = `
INSERT INTO ldap_sync_conflicts (id, domain_id, sync_run_id, object_type, ldap_dn, conflict_type, local_value, ldap_value, resolution, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, 'deferred', now())`

	_, err := r.db.ExecContext(ctx, query, domainID, runID, objectType, ldapDN, conflictType, localValue, ldapValue)
	return err
}

// GetLDAPSyncConflicts lists conflicts for a domain or sync run.
func (r *Repository) GetLDAPSyncConflicts(ctx context.Context, req LDAPSyncConflictListRequest) ([]LDAPSyncConflictView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	query := buildLDAPSyncConflictsSQL(req)
	args := []any{req.DomainID, req.Limit}
	if req.SyncRunID != "" {
		args = append(args, req.SyncRunID)
	}
	if req.Cursor.IsZero() {
		args = append(args, req.Offset)
	} else {
		args = append(args, req.Cursor.CreatedAt, req.Cursor.ID)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conflicts []LDAPSyncConflictView
	for rows.Next() {
		var conflict LDAPSyncConflictView
		err := rows.Scan(&conflict.ID, &conflict.DomainID, &conflict.SyncRunID, &conflict.ObjectType, &conflict.ObjectID, &conflict.LDAPDN, &conflict.ConflictType,
			&conflict.LocalValue, &conflict.LDAPValue, &conflict.Resolution, &conflict.ResolvedAt, &conflict.CreatedAt)
		if err != nil {
			return nil, err
		}
		conflicts = append(conflicts, conflict)
	}
	return conflicts, rows.Err()
}

func buildLDAPSyncConflictsSQL(req LDAPSyncConflictListRequest) string {
	where := "WHERE domain_id = $1"
	nextArg := 3
	if req.SyncRunID != "" {
		where += fmt.Sprintf("\n  AND sync_run_id = $%d::uuid", nextArg)
		nextArg++
	}
	if req.UnresolvedOnly {
		where += "\n  AND resolved_at IS NULL"
	}
	paging := fmt.Sprintf("LIMIT $2 OFFSET $%d", nextArg)
	if !req.Cursor.IsZero() {
		where += fmt.Sprintf("\n  AND (created_at, id) < ($%d::timestamptz, $%d::uuid)", nextArg, nextArg+1)
		paging = "LIMIT $2"
	}
	return `
SELECT id, domain_id, sync_run_id, object_type, object_id, ldap_dn, conflict_type,
       local_value, ldap_value, resolution, resolved_at, created_at
FROM ldap_sync_conflicts
` + where + `
ORDER BY created_at DESC, id DESC
` + paging
}

// ResolveLDAPSyncConflict updates a conflict resolution.
func (r *Repository) ResolveLDAPSyncConflict(ctx context.Context, conflictID uuid.UUID, resolution string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	const query = `
UPDATE ldap_sync_conflicts
SET resolution = $2, resolved_at = now()
WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, conflictID, resolution)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetLDAPSyncRun retrieves a single sync run by ID.
func (r *Repository) GetLDAPSyncRun(ctx context.Context, runID uuid.UUID) (*LDAPSyncRunView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT id, domain_id, sync_type, status, created_count, updated_count, deleted_count,
       conflict_count, error_count, resolution_strategy, started_at, completed_at,
       last_success_at, duration_ms, error_message
FROM ldap_sync_runs
WHERE id = $1`

	var run LDAPSyncRunView
	err := r.db.QueryRowContext(ctx, query, runID).Scan(&run.ID, &run.DomainID, &run.SyncType, &run.Status, &run.CreatedCount, &run.UpdatedCount, &run.DeletedCount,
		&run.ConflictCount, &run.ErrorCount, &run.ResolutionStrategy, &run.StartedAt, &run.CompletedAt,
		&run.LastSuccessAt, &run.DurationMs, &run.ErrorMessage)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// GetLDAPSyncConflict retrieves a single conflict by ID.
func (r *Repository) GetLDAPSyncConflict(ctx context.Context, conflictID uuid.UUID) (*LDAPSyncConflictView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT id, domain_id, sync_run_id, object_type, object_id, ldap_dn, conflict_type,
       local_value, ldap_value, resolution, resolved_at, created_at
FROM ldap_sync_conflicts
WHERE id = $1`

	var conflict LDAPSyncConflictView
	err := r.db.QueryRowContext(ctx, query, conflictID).Scan(&conflict.ID, &conflict.DomainID, &conflict.SyncRunID, &conflict.ObjectType, &conflict.ObjectID, &conflict.LDAPDN, &conflict.ConflictType,
		&conflict.LocalValue, &conflict.LDAPValue, &conflict.Resolution, &conflict.ResolvedAt, &conflict.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conflict, nil
}

// GetLastLDAPSyncTime returns the timestamp of the last successful sync for a domain and type.
func (r *Repository) GetLastLDAPSyncTime(ctx context.Context, domainID uuid.UUID, syncType string) (*time.Time, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT last_success_at
FROM ldap_sync_runs
WHERE domain_id = $1 AND sync_type = $2 AND status = 'success'
ORDER BY last_success_at DESC
LIMIT 1`

	var lastSync *time.Time
	err := r.db.QueryRowContext(ctx, query, domainID, syncType).Scan(&lastSync)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return lastSync, nil
}
