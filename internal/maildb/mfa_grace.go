package maildb

import (
	"context"
	"fmt"
	"time"
)

// SetMFAGraceDeadline sets the MFA enrollment grace deadline for a user.
// After this deadline, FindExpiredMFAGraceUsers will surface the user
// for enforcement if MFA is still not enabled.
func (r *Repository) SetMFAGraceDeadline(ctx context.Context, userID string, deadline time.Time) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	const query = `
INSERT INTO user_mfa_secrets (user_id, secret, mfa_grace_deadline)
VALUES ($1::uuid, '', $2)
ON CONFLICT (user_id) DO UPDATE SET
    mfa_grace_deadline = EXCLUDED.mfa_grace_deadline,
    updated_at         = NOW()`
	_, err := r.db.ExecContext(ctx, query, userID, deadline)
	return err
}

// FindExpiredMFAGraceUsers returns user IDs whose MFA grace period has expired
// but who have not yet enabled MFA (enabled = false).
func (r *Repository) FindExpiredMFAGraceUsers(ctx context.Context, limit int) ([]string, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT user_id::text
FROM user_mfa_secrets
WHERE enabled = FALSE
  AND mfa_grace_deadline IS NOT NULL
  AND mfa_grace_deadline < now()
ORDER BY mfa_grace_deadline
LIMIT $1`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var userIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, id)
	}
	return userIDs, rows.Err()
}

// ClearMFAGraceDeadline nulls out the grace deadline after the user has been
// processed by the enforcement job, preventing repeated enforcement actions.
func (r *Repository) ClearMFAGraceDeadline(ctx context.Context, userID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	const query = `
UPDATE user_mfa_secrets
SET mfa_grace_deadline = NULL, updated_at = NOW()
WHERE user_id = $1::uuid`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}
