package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateRDBMSSyncRun creates a new RDBMS sync run record and returns the run ID.
func (r *Repository) CreateRDBMSSyncRun(ctx context.Context, domainID uuid.UUID, syncType string, metaData *string) (uuid.UUID, error) {
	if r.db == nil {
		return uuid.Nil, fmt.Errorf("database handle is required")
	}

	runID := uuid.New()
	now := time.Now()

	query := `
		INSERT INTO rdbms_sync_runs (id, domain_id, sync_type, status, started_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.ExecContext(ctx, query,
		runID,
		domainID,
		syncType,
		"pending",
		now,
		now,
	)

	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create sync run: %w", err)
	}

	return runID, nil
}

// UpdateRDBMSSyncRun updates an existing RDBMS sync run with results and completion time.
func (r *Repository) UpdateRDBMSSyncRun(ctx context.Context, runID uuid.UUID, status string,
	usersCreated, usersUpdated, usersDeleted, groupsCreated, groupsUpdated, groupsDeleted int,
	conflictCount, errorCount int, errorMsg *string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	now := time.Now()

	query := `
		UPDATE rdbms_sync_runs
		SET status = $2, users_created = $3, users_updated = $4, users_deleted = $5,
		    groups_created = $6, groups_updated = $7, groups_deleted = $8,
		    conflict_count = $9, error_count = $10, error_message = $11, completed_at = $12
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query,
		runID,
		status,
		usersCreated,
		usersUpdated,
		usersDeleted,
		groupsCreated,
		groupsUpdated,
		groupsDeleted,
		conflictCount,
		errorCount,
		errorMsg,
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to update sync run: %w", err)
	}

	return nil
}

// GetRDBMSSyncRuns retrieves a list of RDBMS sync runs for a domain with pagination.
func (r *Repository) GetRDBMSSyncRuns(ctx context.Context, req RDBMSSyncRunListRequest) ([]RDBMSSyncRunView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	query := buildRDBMSSyncRunsSQL(!req.Cursor.IsZero())
	args := []any{req.DomainID, req.Limit}
	if req.Cursor.IsZero() {
		args = append(args, req.Offset)
	} else {
		args = append(args, req.Cursor.CreatedAt, req.Cursor.ID)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query sync runs: %w", err)
	}
	defer rows.Close()

	var runs []RDBMSSyncRunView
	for rows.Next() {
		var run RDBMSSyncRunView
		err := rows.Scan(
			&run.ID,
			&run.DomainID,
			&run.SyncType,
			&run.Status,
			&run.UsersCreated,
			&run.UsersUpdated,
			&run.UsersDeleted,
			&run.GroupsCreated,
			&run.GroupsUpdated,
			&run.GroupsDeleted,
			&run.ConflictCount,
			&run.ErrorCount,
			&run.ErrorMessage,
			&run.StartedAt,
			&run.CompletedAt,
			&run.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sync run: %w", err)
		}
		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading sync runs: %w", err)
	}

	return runs, nil
}

func buildRDBMSSyncRunsSQL(hasCursor bool) string {
	where := "WHERE domain_id = $1"
	if hasCursor {
		where += "\n\t\t  AND (created_at, id) < ($3::timestamptz, $4::uuid)"
	}
	paging := "LIMIT $2 OFFSET $3"
	if hasCursor {
		paging = "LIMIT $2"
	}
	return `
		SELECT id, domain_id, sync_type, status, users_created, users_updated, users_deleted,
		       groups_created, groups_updated, groups_deleted, conflict_count, error_count, error_message,
		       started_at, completed_at, created_at
		FROM rdbms_sync_runs
		` + where + `
		ORDER BY created_at DESC, id DESC
		` + paging + `
	`
}

// GetRDBMSSyncRun retrieves a single RDBMS sync run by ID.
func (r *Repository) GetRDBMSSyncRun(ctx context.Context, runID uuid.UUID) (*RDBMSSyncRunView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	query := `
		SELECT id, domain_id, sync_type, status, users_created, users_updated, users_deleted,
		       groups_created, groups_updated, groups_deleted, conflict_count, error_count, error_message,
		       started_at, completed_at, created_at
		FROM rdbms_sync_runs
		WHERE id = $1
	`

	var run RDBMSSyncRunView
	err := r.db.QueryRowContext(ctx, query, runID).Scan(
		&run.ID,
		&run.DomainID,
		&run.SyncType,
		&run.Status,
		&run.UsersCreated,
		&run.UsersUpdated,
		&run.UsersDeleted,
		&run.GroupsCreated,
		&run.GroupsUpdated,
		&run.GroupsDeleted,
		&run.ConflictCount,
		&run.ErrorCount,
		&run.ErrorMessage,
		&run.StartedAt,
		&run.CompletedAt,
		&run.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sync run not found")
		}
		return nil, fmt.Errorf("failed to query sync run: %w", err)
	}

	return &run, nil
}

// AddRDBMSSyncConflict adds a conflict record for manual review.
func (r *Repository) AddRDBMSSyncConflict(ctx context.Context, domainID, syncRunID uuid.UUID, conflictType, localData, remoteData string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	conflictID := uuid.New()
	now := time.Now()

	query := `
		INSERT INTO rdbms_sync_conflicts (id, domain_id, sync_run_id, conflict_type, local_data, remote_data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		conflictID,
		domainID,
		syncRunID,
		conflictType,
		localData,
		remoteData,
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to add conflict: %w", err)
	}

	return nil
}

// GetRDBMSSyncConflicts retrieves unresolved RDBMS sync conflicts with optional filtering.
func (r *Repository) GetRDBMSSyncConflicts(ctx context.Context, req RDBMSSyncConflictListRequest) ([]RDBMSSyncConflictView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	query := `
		SELECT id, domain_id, sync_run_id, conflict_type, local_data, remote_data, resolution, resolved_at, created_at
		FROM rdbms_sync_conflicts
		WHERE domain_id = $1
	`

	args := []interface{}{req.DomainID}

	if req.UnresolvedOnly {
		query += " AND resolution IS NULL"
	}

	query += " ORDER BY created_at DESC LIMIT $2 OFFSET $3"
	args = append(args, req.Limit, req.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query conflicts: %w", err)
	}
	defer rows.Close()

	var conflicts []RDBMSSyncConflictView
	for rows.Next() {
		var conflict RDBMSSyncConflictView
		err := rows.Scan(
			&conflict.ID,
			&conflict.DomainID,
			&conflict.SyncRunID,
			&conflict.ConflictType,
			&conflict.LocalData,
			&conflict.RemoteData,
			&conflict.Resolution,
			&conflict.ResolvedAt,
			&conflict.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conflict: %w", err)
		}
		conflicts = append(conflicts, conflict)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading conflicts: %w", err)
	}

	return conflicts, nil
}

// GetRDBMSSyncConflict retrieves a single RDBMS sync conflict by ID.
func (r *Repository) GetRDBMSSyncConflict(ctx context.Context, conflictID uuid.UUID) (*RDBMSSyncConflictView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	query := `
		SELECT id, domain_id, sync_run_id, conflict_type, local_data, remote_data, resolution, resolved_at, created_at
		FROM rdbms_sync_conflicts
		WHERE id = $1
	`

	var conflict RDBMSSyncConflictView
	err := r.db.QueryRowContext(ctx, query, conflictID).Scan(
		&conflict.ID,
		&conflict.DomainID,
		&conflict.SyncRunID,
		&conflict.ConflictType,
		&conflict.LocalData,
		&conflict.RemoteData,
		&conflict.Resolution,
		&conflict.ResolvedAt,
		&conflict.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("conflict not found")
		}
		return nil, fmt.Errorf("failed to query conflict: %w", err)
	}

	return &conflict, nil
}

// ResolveRDBMSSyncConflict marks a conflict as resolved with a resolution strategy.
func (r *Repository) ResolveRDBMSSyncConflict(ctx context.Context, conflictID uuid.UUID, resolution string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	now := time.Now()

	query := `
		UPDATE rdbms_sync_conflicts
		SET resolution = $2, resolved_at = $3
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, conflictID, resolution, now)
	if err != nil {
		return fmt.Errorf("failed to resolve conflict: %w", err)
	}

	return nil
}

// GetLastRDBMSSyncTime returns the timestamp of the last successful RDBMS sync for incremental sync.
func (r *Repository) GetLastRDBMSSyncTime(ctx context.Context, domainID uuid.UUID, syncType string) (*time.Time, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	query := `
		SELECT MAX(completed_at)
		FROM rdbms_sync_runs
		WHERE domain_id = $1 AND sync_type = $2 AND status = 'success' AND completed_at IS NOT NULL
	`

	var lastTime *time.Time
	err := r.db.QueryRowContext(ctx, query, domainID, syncType).Scan(&lastTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query last sync time: %w", err)
	}

	return lastTime, nil
}
