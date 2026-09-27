package backup

import (
	"context"
	"database/sql"
	"fmt"
)

// TargetState describes the restore target database's readiness for a restore.
type TargetState struct {
	// IsEmpty is true when the target has no user tables in the public schema.
	IsEmpty bool
	// MigrationVersion is the applied goose migration version, or 0 when the
	// goose version table is absent (a fresh database).
	MigrationVersion int64
}

// InspectTarget reports whether the target database is empty and its current
// migration version. It tolerates a database that has never been migrated:
// a missing goose_db_version table yields MigrationVersion=0 and does not error.
//
// "Empty" means the public schema contains no base tables. This is the signal
// [CheckRestoreGuards] uses to refuse clobbering a populated database without
// --force.
func InspectTarget(ctx context.Context, db *sql.DB) (TargetState, error) {
	if db == nil {
		return TargetState{}, fmt.Errorf("database handle is required")
	}

	var tableCount int
	err := db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_type = 'BASE TABLE'`).Scan(&tableCount)
	if err != nil {
		return TargetState{}, fmt.Errorf("count target tables: %w", err)
	}

	state := TargetState{IsEmpty: tableCount == 0}

	// Detect the goose version table without failing when it is absent.
	var hasGoose bool
	if err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'goose_db_version'
		)`).Scan(&hasGoose); err != nil {
		return TargetState{}, fmt.Errorf("detect goose version table: %w", err)
	}
	if !hasGoose {
		return state, nil
	}

	var version sql.NullInt64
	if err := db.QueryRowContext(ctx, `
		SELECT max(version_id) FROM goose_db_version WHERE is_applied = true`).Scan(&version); err != nil {
		return TargetState{}, fmt.Errorf("read target migration version: %w", err)
	}
	if version.Valid {
		state.MigrationVersion = version.Int64
	}
	return state, nil
}
