package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/apikeys"
	"github.com/lib/pq"
)

const (
	MCPPermissionModeBasic  = "basic"
	MCPPermissionModeBypass = "bypass"
)

type UserMCPAccessKey struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	DomainID       string     `json:"domain_id"`
	Name           string     `json:"name"`
	TokenSuffix    string     `json:"token_suffix"`
	Scopes         []string   `json:"scopes"`
	PermissionMode string     `json:"permission_mode"`
	AllowedCIDRs   []string   `json:"allowed_cidrs"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	Revoked        bool       `json:"revoked"`
}

type CreateUserMCPAccessKeyRequest struct {
	UserID         string     `json:"user_id"`
	Name           string     `json:"name"`
	Scopes         []string   `json:"scopes"`
	PermissionMode string     `json:"permission_mode"`
	AllowedCIDRs   []string   `json:"allowed_cidrs"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}

type CreatedUserMCPAccessKey struct {
	Key   UserMCPAccessKey `json:"key"`
	Token string           `json:"token"`
}

type domainMCPPolicy struct {
	Enabled             bool     `json:"enabled"`
	AllowBypassMode     bool     `json:"allow_bypass_mode"`
	AllowUserAccessKeys bool     `json:"allow_user_access_keys"`
	AllowedScopes       []string `json:"allowed_scopes"`
}

func (r *Repository) ListUserMCPAccessKeys(ctx context.Context, userID string) ([]UserMCPAccessKey, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id::text, user_id::text, domain_id::text, name, token_suffix, scopes, permission_mode, allowed_cidrs,
       expires_at, created_at, last_used_at, revoked
FROM user_mcp_access_keys
WHERE user_id = $1::uuid
ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user mcp access keys: %w", err)
	}
	defer rows.Close()
	var keys []UserMCPAccessKey
	for rows.Next() {
		key, err := scanUserMCPAccessKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (r *Repository) CreateUserMCPAccessKey(ctx context.Context, req CreateUserMCPAccessKeyRequest) (CreatedUserMCPAccessKey, error) {
	req.UserID = strings.TrimSpace(req.UserID)
	req.Name = strings.TrimSpace(req.Name)
	if req.UserID == "" {
		return CreatedUserMCPAccessKey{}, fmt.Errorf("user_id is required")
	}
	if req.Name == "" {
		return CreatedUserMCPAccessKey{}, fmt.Errorf("name is required")
	}
	req.PermissionMode = normalizeMCPPermissionMode(req.PermissionMode)
	scopes := normalizeMCPScopes(req.Scopes)
	if len(scopes) == 0 {
		scopes = []string{"mail:read"}
	}
	policy, err := r.userDomainMCPPolicy(ctx, req.UserID)
	if err != nil {
		return CreatedUserMCPAccessKey{}, err
	}
	if !policy.Enabled {
		return CreatedUserMCPAccessKey{}, fmt.Errorf("mcp is disabled for this domain")
	}
	if !policy.AllowUserAccessKeys {
		return CreatedUserMCPAccessKey{}, fmt.Errorf("user mcp access keys are disabled for this domain")
	}
	if req.PermissionMode == MCPPermissionModeBypass && !policy.AllowBypassMode {
		return CreatedUserMCPAccessKey{}, fmt.Errorf("bypass mode is disabled for this domain")
	}
	if err := ensureUserMCPScopesAllowed(scopes, policy.AllowedScopes); err != nil {
		return CreatedUserMCPAccessKey{}, err
	}
	cidrs := normalizeStringSlice(req.AllowedCIDRs)
	token, err := apikeys.GenerateUserMCPKey()
	if err != nil {
		return CreatedUserMCPAccessKey{}, err
	}
	tokenSuffix := token
	if len(tokenSuffix) > 8 {
		tokenSuffix = tokenSuffix[len(tokenSuffix)-8:]
	}
	row := r.db.QueryRowContext(ctx, `
WITH u AS (
  SELECT id, domain_id FROM users WHERE id = $1::uuid
)
INSERT INTO user_mcp_access_keys (user_id, domain_id, key_hash, token_suffix, name, scopes, permission_mode, allowed_cidrs, expires_at)
SELECT u.id, u.domain_id, $2, $3, $4, $5, $6, $7, $8
FROM u
RETURNING id::text, user_id::text, domain_id::text, name, token_suffix, scopes, permission_mode, allowed_cidrs,
          expires_at, created_at, last_used_at, revoked`,
		req.UserID,
		apikeys.HashKey(token),
		tokenSuffix,
		req.Name,
		pq.Array(scopes),
		req.PermissionMode,
		pq.Array(cidrs),
		req.ExpiresAt,
	)
	key, err := scanUserMCPAccessKey(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return CreatedUserMCPAccessKey{}, fmt.Errorf("user not found")
		}
		return CreatedUserMCPAccessKey{}, fmt.Errorf("create user mcp access key: %w", err)
	}
	return CreatedUserMCPAccessKey{Key: key, Token: token}, nil
}

func (r *Repository) userDomainMCPPolicy(ctx context.Context, userID string) (domainMCPPolicy, error) {
	policy := domainMCPPolicy{
		Enabled:             true,
		AllowBypassMode:     true,
		AllowUserAccessKeys: true,
		AllowedScopes:       []string{"mail:read", "mail:write", "mail:send", "mail:manage", "contacts:read", "contacts:write", "contacts:manage", "drive:read", "drive:write", "drive:manage", "calendar:read", "calendar:write", "calendar:manage"},
	}
	var raw []byte
	err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(rc.value, '{}'::jsonb)
FROM users u
LEFT JOIN runtime_config rc
  ON rc.scope_type = 'domain'
 AND rc.scope_id = u.domain_id
 AND rc.key = 'mcp.policy'
WHERE u.id = $1::uuid`, userID).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return domainMCPPolicy{}, fmt.Errorf("user not found")
		}
		return domainMCPPolicy{}, fmt.Errorf("load domain mcp policy: %w", err)
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &policy); err != nil {
			return domainMCPPolicy{}, fmt.Errorf("domain mcp policy is invalid")
		}
	}
	return policy, nil
}

func ensureUserMCPScopesAllowed(scopes, allowedScopes []string) error {
	allowed := map[string]struct{}{}
	for _, scope := range normalizeMCPScopes(allowedScopes) {
		allowed[scope] = struct{}{}
	}
	if len(allowed) == 0 {
		return fmt.Errorf("no mcp scopes are allowed for this domain")
	}
	for _, scope := range scopes {
		if _, ok := allowed[scope]; ok {
			continue
		}
		parts := strings.SplitN(scope, ":", 2)
		if len(parts) == 2 {
			if _, ok := allowed[parts[0]+":manage"]; ok {
				continue
			}
		}
		return fmt.Errorf("mcp scope %q is not allowed for this domain", scope)
	}
	return nil
}

func (r *Repository) RevokeUserMCPAccessKey(ctx context.Context, userID, id string) (UserMCPAccessKey, error) {
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	if userID == "" || id == "" {
		return UserMCPAccessKey{}, fmt.Errorf("user_id and id are required")
	}
	row := r.db.QueryRowContext(ctx, `
UPDATE user_mcp_access_keys
SET revoked = true, revoked_at = now()
WHERE user_id = $1::uuid AND id = $2::uuid AND revoked = false
RETURNING id::text, user_id::text, domain_id::text, name, token_suffix, scopes, permission_mode, allowed_cidrs,
          expires_at, created_at, last_used_at, revoked`, userID, id)
	key, err := scanUserMCPAccessKey(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return UserMCPAccessKey{}, fmt.Errorf("mcp access key not found")
		}
		return UserMCPAccessKey{}, fmt.Errorf("revoke user mcp access key: %w", err)
	}
	return key, nil
}

type userMCPAccessKeyScanner interface {
	Scan(dest ...any) error
}

func scanUserMCPAccessKey(row userMCPAccessKeyScanner) (UserMCPAccessKey, error) {
	var key UserMCPAccessKey
	var expiresAt, lastUsedAt sql.NullTime
	if err := row.Scan(
		&key.ID,
		&key.UserID,
		&key.DomainID,
		&key.Name,
		&key.TokenSuffix,
		pq.Array(&key.Scopes),
		&key.PermissionMode,
		pq.Array(&key.AllowedCIDRs),
		&expiresAt,
		&key.CreatedAt,
		&lastUsedAt,
		&key.Revoked,
	); err != nil {
		return UserMCPAccessKey{}, err
	}
	if expiresAt.Valid {
		key.ExpiresAt = &expiresAt.Time
	}
	if lastUsedAt.Valid {
		key.LastUsedAt = &lastUsedAt.Time
	}
	return key, nil
}

func normalizeMCPPermissionMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case MCPPermissionModeBypass:
		return MCPPermissionModeBypass
	default:
		return MCPPermissionModeBasic
	}
}

func normalizeMCPScopes(values []string) []string {
	return normalizeStringSlice(values)
}

func normalizeStringSlice(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		value = strings.ReplaceAll(value, ".", ":")
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
