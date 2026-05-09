package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// SSOUserInfo is the subset of user identity needed by the SSO flow handlers.
type SSOUserInfo struct {
	UserID    string
	DomainID  string
	CompanyID string
	Email     string
}

// GetUserByEmail looks up an active user by their primary or alias email address.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (SSOUserInfo, error) {
	if r.db == nil {
		return SSOUserInfo{}, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT ua.user_id::text, ua.domain_id::text, d.company_id::text, ua.address
FROM user_addresses ua
JOIN users u ON u.id = ua.user_id
JOIN domains d ON d.id = ua.domain_id
WHERE lower(ua.address) = lower($1)
  AND u.status = 'active'
  AND d.status = 'active'
LIMIT 1`
	var info SSOUserInfo
	err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(email)).Scan(
		&info.UserID, &info.DomainID, &info.CompanyID, &info.Email,
	)
	if err == sql.ErrNoRows {
		return SSOUserInfo{}, fmt.Errorf("user not found: %s", email)
	}
	return info, err
}

// JITCreateSSOUser creates a new user account for SSO just-in-time provisioning.
// The username is derived from the local part of the email address.
func (r *Repository) JITCreateSSOUser(ctx context.Context, email, domainID, displayName string) (SSOUserInfo, error) {
	if r.db == nil {
		return SSOUserInfo{}, fmt.Errorf("database handle is required")
	}
	at := strings.Index(email, "@")
	if at < 0 {
		return SSOUserInfo{}, fmt.Errorf("invalid email address: %s", email)
	}
	username := email[:at]
	if displayName == "" {
		displayName = username
	}
	view, err := r.CreateUser(ctx, CreateUserRequest{
		DomainID:    domainID,
		Username:    username,
		DisplayName: displayName,
		Address:     email,
	})
	if err != nil {
		return SSOUserInfo{}, fmt.Errorf("jit provision %s: %w", email, err)
	}
	return SSOUserInfo{
		UserID:   view.ID,
		DomainID: view.DomainID,
		Email:    email,
	}, nil
}
