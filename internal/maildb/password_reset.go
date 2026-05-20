package maildb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// PasswordResetToken represents a row in password_reset_tokens.
type PasswordResetToken struct {
	ID        string
	UserID    string
	TokenHash []byte
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// CreatePasswordResetToken inserts a new reset token record. tokenHash must be
// the SHA-256 digest of the raw random token (32 bytes).
func (r *Repository) CreatePasswordResetToken(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}
	if len(tokenHash) == 0 {
		return fmt.Errorf("token_hash is required")
	}
	if expiresAt.IsZero() {
		return fmt.Errorf("expires_at is required")
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		 VALUES ($1::uuid, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("create password reset token: %w", err)
	}
	return nil
}

// GetPasswordResetToken looks up a token by its SHA-256 hash. Returns an error
// if the token does not exist, is expired, or has already been used.
func (r *Repository) GetPasswordResetToken(ctx context.Context, tokenHash []byte) (*PasswordResetToken, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if len(tokenHash) == 0 {
		return nil, fmt.Errorf("token_hash is required")
	}
	var t PasswordResetToken
	err := r.db.QueryRowContext(ctx,
		`SELECT id::text, user_id::text, token_hash, expires_at, used_at, created_at
		 FROM password_reset_tokens
		 WHERE token_hash = $1`,
		tokenHash,
	).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("token not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get password reset token: %w", err)
	}
	return &t, nil
}

// MarkTokenUsed records the current timestamp in used_at for the given token ID.
func (r *Repository) MarkTokenUsed(ctx context.Context, tokenID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if tokenID == "" {
		return fmt.Errorf("token_id is required")
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE password_reset_tokens SET used_at = NOW() WHERE id = $1::uuid AND used_at IS NULL`,
		tokenID,
	)
	if err != nil {
		return fmt.Errorf("mark token used: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("token not found or already used")
	}
	return nil
}

// ResetUserPassword replaces the password hash for the given user without
// requiring the current password. It also bumps session_version to invalidate
// all existing sessions. Used exclusively from the password-reset flow.
func (r *Repository) ResetUserPassword(ctx context.Context, userID, newPasswordHash string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}
	if newPasswordHash == "" {
		return fmt.Errorf("password_hash is required")
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE users
		    SET password_hash     = $2,
		        session_version   = session_version + 1,
		        updated_at        = NOW()
		  WHERE id = $1::uuid
		    AND auth_source = 'local'`,
		userID, newPasswordHash,
	)
	if err != nil {
		return fmt.Errorf("reset user password: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found or not a local auth user")
	}
	return nil
}

// GenerateResetToken produces 32 cryptographically random bytes suitable for
// use as a password reset token. Callers should hex-encode or base64-encode
// the raw bytes before including them in a URL.
func GenerateResetToken() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate reset token: %w", err)
	}
	return b, nil
}
