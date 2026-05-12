package maildb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/auth"
)

type UserProfile struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	QuotaUsed   int64  `json:"quota_used"`
	QuotaLimit  *int64 `json:"quota_limit"`
}

// GetUserProfile returns the profile for the authenticated user.
func (r *Repository) GetUserProfile(ctx context.Context, userID string) (UserProfile, error) {
	if r.db == nil {
		return UserProfile{}, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT u.id::text, u.display_name, ua.address, u.quota_used, u.quota_limit
FROM users u
JOIN user_addresses ua ON ua.user_id = u.id AND ua.is_primary = true
WHERE u.id = $1::uuid
LIMIT 1`

	var p UserProfile
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&p.UserID,
		&p.DisplayName,
		&p.Email,
		&p.QuotaUsed,
		&p.QuotaLimit,
	)
	if err == sql.ErrNoRows {
		return UserProfile{}, fmt.Errorf("user not found")
	}
	if err != nil {
		return UserProfile{}, fmt.Errorf("get user profile: %w", err)
	}
	return p, nil
}

// ChangeUserPassword verifies currentPassword and replaces the stored hash.
// On success the session_version is bumped, invalidating all existing tokens.
func (r *Repository) ChangeUserPassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if strings.TrimSpace(newPassword) == "" {
		return fmt.Errorf("new password is required")
	}
	if len(newPassword) < 8 {
		return fmt.Errorf("new password must be at least 8 characters")
	}

	// Fetch current hash
	var currentHash string
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(password_hash, '') FROM users WHERE id = $1::uuid AND auth_source = 'local'`,
		userID,
	).Scan(&currentHash)
	if err == sql.ErrNoRows {
		return fmt.Errorf("user not found or external auth")
	}
	if err != nil {
		return fmt.Errorf("fetch password hash: %w", err)
	}

	if !auth.VerifyPasswordHash(currentPassword, currentHash) {
		return fmt.Errorf("current password is incorrect")
	}

	// Generate new hash
	var salt [32]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}
	newHash, err := auth.HashPasswordPBKDF2SHA256(newPassword, salt[:], 0)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		`UPDATE users SET password_hash = $2, session_version = session_version + 1, updated_at = now() WHERE id = $1::uuid`,
		userID, newHash,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}
