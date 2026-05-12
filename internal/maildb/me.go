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

type UserAddress struct {
	ID        string `json:"id"`
	Address   string `json:"address"`
	IsPrimary bool   `json:"is_primary"`
}

// ListUserAddresses returns all email addresses for the authenticated user,
// primary address first.
func (r *Repository) ListUserAddresses(ctx context.Context, userID string) ([]UserAddress, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id::text, address, is_primary FROM user_addresses WHERE user_id = $1::uuid ORDER BY is_primary DESC, address ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list user addresses: %w", err)
	}
	defer rows.Close()
	var addrs []UserAddress
	for rows.Next() {
		var a UserAddress
		if err := rows.Scan(&a.ID, &a.Address, &a.IsPrimary); err != nil {
			return nil, fmt.Errorf("scan user address: %w", err)
		}
		addrs = append(addrs, a)
	}
	return addrs, rows.Err()
}

// UpdateUserDisplayName sets the display_name for the authenticated user.
func (r *Repository) UpdateUserDisplayName(ctx context.Context, userID, displayName string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return fmt.Errorf("display_name is required")
	}
	if len(displayName) > 255 {
		return fmt.Errorf("display_name is too long")
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET display_name = $2, updated_at = now() WHERE id = $1::uuid`,
		userID, displayName,
	)
	if err != nil {
		return fmt.Errorf("update display name: %w", err)
	}
	return nil
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
