package maildb

import (
	"errors"
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type UserStorageScope struct {
	CompanyID string
	DomainID  string
	UserID    string
}

func (r *Repository) UserStorageScope(ctx context.Context, userID string) (UserStorageScope, error) {
	if r == nil || r.db == nil {
		return UserStorageScope{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return UserStorageScope{}, fmt.Errorf("user_id is required")
	}
	const query = `
SELECT d.company_id::text, d.id::text, u.id::text
FROM users u
JOIN domains d ON d.id = u.domain_id
WHERE u.id = $1::uuid
  AND u.status = 'active'
  AND d.status = 'active'`
	var scope UserStorageScope
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&scope.CompanyID, &scope.DomainID, &scope.UserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserStorageScope{}, fmt.Errorf("active user not found")
		}
		return UserStorageScope{}, fmt.Errorf("lookup user storage scope: %w", err)
	}
	return scope, nil
}
