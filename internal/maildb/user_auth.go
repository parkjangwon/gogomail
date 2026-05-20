package maildb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
    lower(u.username) = lower($1)
    OR lower(ua.address) = lower($2)
  )
ORDER BY ua.is_primary DESC
LIMIT 1`

	var user AuthenticatedUser
	var passwordHash string
	var companyStatus string
	err := r.db.QueryRowContext(ctx, query, normalized, normalizedAddress).Scan(
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
	if !auth.VerifyPasswordHash(password, passwordHash) {
		return AuthenticatedUser{}, fmt.Errorf("invalid credentials")
	}
	return user, nil
}
