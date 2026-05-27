package maildb

import (
	"errors"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

const userRefreshTokenTTL = 30 * 24 * time.Hour

type RotatedUserRefreshToken struct {
	Token string
	User  AuthenticatedUser
}

func (r *Repository) CreateUserRefreshToken(ctx context.Context, userID string) (string, error) {
	if r.db == nil {
		return "", fmt.Errorf("database handle is required")
	}
	token, hash, err := newRefreshToken()
	if err != nil {
		return "", err
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO user_refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)`, userID, hash[:], time.Now().UTC().Add(userRefreshTokenTTL))
	if err != nil {
		return "", fmt.Errorf("create user refresh token: %w", err)
	}
	return token, nil
}

func (r *Repository) RotateUserRefreshToken(ctx context.Context, token string) (RotatedUserRefreshToken, error) {
	if r.db == nil {
		return RotatedUserRefreshToken{}, fmt.Errorf("database handle is required")
	}
	_, oldHash, err := decodeRefreshToken(token)
	if err != nil {
		return RotatedUserRefreshToken{}, err
	}
	newToken, newHash, err := newRefreshToken()
	if err != nil {
		return RotatedUserRefreshToken{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return RotatedUserRefreshToken{}, fmt.Errorf("begin refresh token transaction: %w", err)
	}
	defer tx.Rollback()

	var user AuthenticatedUser
	err = tx.QueryRowContext(ctx, `
UPDATE user_refresh_tokens rt
SET revoked_at = now()
FROM users u
JOIN domains d ON d.id = u.domain_id
JOIN companies c ON c.id = d.company_id
WHERE rt.user_id = u.id
  AND rt.token_hash = $1
  AND rt.revoked_at IS NULL
  AND rt.expires_at > now()
  AND u.status = 'active'
  AND d.status = 'active'
  AND c.status <> 'suspended'
RETURNING u.id::text, u.domain_id::text, d.company_id::text, u.session_version, u.must_change_password`,
		oldHash[:],
	).Scan(&user.UserID, &user.DomainID, &user.CompanyID, &user.SessionVersion, &user.MustChangePassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RotatedUserRefreshToken{}, fmt.Errorf("refresh token not found")
		}
		return RotatedUserRefreshToken{}, fmt.Errorf("rotate user refresh token: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO user_refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)`, user.UserID, newHash[:], time.Now().UTC().Add(userRefreshTokenTTL)); err != nil {
		return RotatedUserRefreshToken{}, fmt.Errorf("create rotated refresh token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return RotatedUserRefreshToken{}, fmt.Errorf("commit refresh token rotation: %w", err)
	}
	return RotatedUserRefreshToken{Token: newToken, User: user}, nil
}

func newRefreshToken() (string, [32]byte, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", [32]byte{}, fmt.Errorf("generate refresh token: %w", err)
	}
	token := hex.EncodeToString(raw[:])
	return token, sha256.Sum256([]byte(token)), nil
}

func decodeRefreshToken(token string) (string, [32]byte, error) {
	raw, err := hex.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return "", [32]byte{}, fmt.Errorf("invalid refresh token")
	}
	return token, sha256.Sum256([]byte(token)), nil
}
