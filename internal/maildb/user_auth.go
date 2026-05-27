package maildb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/mail"
)

// ErrCompanySuspended is returned by AuthenticateUser when the user's company
// has been suspended, blocking all logins for that organisation.
var ErrCompanySuspended = errors.New("company suspended")

type AuthenticatedUser struct {
	UserID             string
	DomainID           string
	CompanyID          string
	SessionVersion     int64
	MustChangePassword bool
}

func (r *Repository) AuthenticateUser(ctx context.Context, email, password string) (AuthenticatedUser, error) {
	normalized := strings.TrimSpace(email)
	normalizedUsername := strings.ToLower(normalized)
	normalizedAddress := normalized
	if strings.Contains(normalized, "@") {
		addr, err := mail.NormalizeAddress(normalized)
		if err == nil {
			normalizedAddress = addr
		}
	}
	const query = `
SELECT u.id::text, u.domain_id::text, d.company_id::text, u.session_version, u.must_change_password,
       COALESCE(u.password_hash, ''), COALESCE(c.status, '')
FROM users u
JOIN domains d ON d.id = u.domain_id
JOIN companies c ON c.id = d.company_id
JOIN user_addresses ua ON ua.user_id = u.id
WHERE u.status = 'active'
  AND d.status = 'active'
  AND u.auth_source = 'local'
  AND (
    lower(u.username) = $1
    OR ua.address_ace = $2
  )
ORDER BY ua.is_primary DESC
LIMIT 1`

	var user AuthenticatedUser
	var passwordHash string
	var companyStatus string
	err := r.db.QueryRowContext(ctx, query, normalizedUsername, normalizedAddress).Scan(
		&user.UserID,
		&user.DomainID,
		&user.CompanyID,
		&user.SessionVersion,
		&user.MustChangePassword,
		&passwordHash,
		&companyStatus,
	)
	if err == sql.ErrNoRows {
		return AuthenticatedUser{}, fmt.Errorf("invalid credentials")
	}
	if err != nil {
		return AuthenticatedUser{}, fmt.Errorf("authenticate user: %w", err)
	}
	if companyStatus == "suspended" {
		return AuthenticatedUser{}, ErrCompanySuspended
	}
	verified, needsUpgrade := auth.VerifyPasswordHashResult(password, passwordHash)
	if !verified {
		return AuthenticatedUser{}, fmt.Errorf("invalid credentials")
	}
	if needsUpgrade {
		// Run async so 210k PBKDF2 iterations + DB write don't block the login
		// response. context.WithoutCancel lets the goroutine outlive the request.
		upgradeCtx := context.WithoutCancel(ctx)
		upgradeUserID := user.UserID
		upgradePwd := password
		go r.upgradePasswordHash(upgradeCtx, upgradeUserID, upgradePwd)
	}
	return user, nil
}

// upgradePasswordHash re-hashes password with PBKDF2-SHA256 and updates the DB.
// Best-effort: any error is logged but does not affect the caller.
func (r *Repository) upgradePasswordHash(ctx context.Context, userID, password string) {
	newHash, err := auth.HashPasswordPBKDF2SHA256(password, auth.GenerateSalt(32), 210_000)
	if err != nil {
		slog.WarnContext(ctx, "password hash upgrade: hash generation failed", "user_id", userID)
		return
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1::uuid`,
		userID, newHash)
	if err != nil {
		slog.WarnContext(ctx, "password hash upgrade: db update failed", "user_id", userID)
		return
	}
	slog.InfoContext(ctx, "password hash upgraded to pbkdf2-sha256", "user_id", userID)
}
