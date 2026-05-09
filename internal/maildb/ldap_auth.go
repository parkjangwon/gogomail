package maildb

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/mail"
)

type LDAPAuthenticator interface {
	AuthenticateLDAP(ctx context.Context, username, password string) (bool, error)
}

func (r *Repository) AuthenticateLDAP(ctx context.Context, username, password string) (bool, error) {
	if r.db == nil {
		return false, nil
	}
	normalized := strings.TrimSpace(username)
	normalizedAddress := normalized
	if strings.Contains(normalized, "@") {
		addr, err := mail.NormalizeAddress(normalized)
		if err == nil {
			normalizedAddress = addr
		}
	}
	const query = `
SELECT COALESCE(u.password_hash, '')
FROM users u
JOIN domains d ON d.id = u.domain_id
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
	var passwordHash string
	err := r.db.QueryRowContext(ctx, query, normalized, normalizedAddress).Scan(&passwordHash)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return auth.VerifyPasswordHash(password, passwordHash), nil
}
