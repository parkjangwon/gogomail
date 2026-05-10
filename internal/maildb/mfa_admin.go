package maildb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// UserMFAStatus holds the MFA enrollment state for a single user.
type UserMFAStatus struct {
	UserID    string `json:"user_id"`
	Enabled   bool   `json:"enabled"`
	Enrolled  bool   `json:"enrolled"` // true if a row exists in user_mfa_secrets
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// MFAStats holds MFA adoption statistics for a company.
type MFAStats struct {
	Total       int `json:"total"`
	Enabled     int `json:"enabled"`
	Enrolled    int `json:"enrolled"` // has a row but not enabled
	NotEnrolled int `json:"not_enrolled"`
}

// GetUserMFAStatus returns the MFA status for a single user.
// If the user has no row in user_mfa_secrets, returns Enrolled: false, Enabled: false.
func (r *Repository) GetUserMFAStatus(ctx context.Context, userID string) (UserMFAStatus, error) {
	if r.db == nil {
		return UserMFAStatus{}, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT user_id::text, enabled, created_at::text, updated_at::text
FROM user_mfa_secrets
WHERE user_id = $1::uuid`

	var status UserMFAStatus
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&status.UserID,
		&status.Enabled,
		&status.CreatedAt,
		&status.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return UserMFAStatus{UserID: userID, Enabled: false, Enrolled: false}, nil
	}
	if err != nil {
		return UserMFAStatus{}, err
	}
	status.Enrolled = true
	return status, nil
}

// ResetUserMFA deletes the MFA secret for a user so they must re-enroll.
func (r *Repository) ResetUserMFA(ctx context.Context, userID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	const query = `DELETE FROM user_mfa_secrets WHERE user_id = $1::uuid`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

// GetMFAStats returns MFA adoption statistics for all users in a company.
func (r *Repository) GetMFAStats(ctx context.Context, companyID string) (MFAStats, error) {
	if r.db == nil {
		return MFAStats{}, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT
    COUNT(u.id) AS total,
    COUNT(m.user_id) FILTER (WHERE m.enabled = true)  AS enabled,
    COUNT(m.user_id) FILTER (WHERE m.enabled = false) AS enrolled,
    COUNT(u.id) - COUNT(m.user_id)                    AS not_enrolled
FROM users u
JOIN domains d ON u.domain_id = d.id
LEFT JOIN user_mfa_secrets m ON m.user_id = u.id
WHERE d.company_id = $1::uuid`

	var stats MFAStats
	err := r.db.QueryRowContext(ctx, query, companyID).Scan(
		&stats.Total,
		&stats.Enabled,
		&stats.Enrolled,
		&stats.NotEnrolled,
	)
	if err != nil {
		return MFAStats{}, err
	}
	return stats, nil
}
