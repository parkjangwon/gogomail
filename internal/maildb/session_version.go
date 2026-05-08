package maildb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (r *Repository) SessionVersionFor(ctx context.Context, userID string) (int64, error) {
	var ver int64
	err := r.db.QueryRowContext(ctx,
		`SELECT session_version FROM users WHERE id = $1`, userID,
	).Scan(&ver)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("user not found: %s", userID)
	}
	if err != nil {
		return 0, fmt.Errorf("session version for user %s: %w", userID, err)
	}
	return ver, nil
}

func (r *Repository) IncrementSessionVersion(ctx context.Context, userID string) (int64, error) {
	var newVer int64
	err := r.db.QueryRowContext(ctx,
		`UPDATE users
		    SET session_version = session_version + 1,
		        updated_at      = now()
		  WHERE id = $1
		  RETURNING session_version`,
		userID,
	).Scan(&newVer)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("user not found: %s", userID)
	}
	if err != nil {
		return 0, fmt.Errorf("increment session version for user %s: %w", userID, err)
	}
	return newVer, nil
}
