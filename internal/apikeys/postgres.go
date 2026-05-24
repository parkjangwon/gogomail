package apikeys

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"

	"github.com/lib/pq"
)

type PostgresVerifier struct {
	db *sql.DB
}

func NewPostgresVerifier(db *sql.DB) PostgresVerifier {
	return PostgresVerifier{db: db}
}

func (v PostgresVerifier) Verify(ctx context.Context, keyHash string, ip net.IP) (*KeyInfo, error) {
	if v.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	keyHash = strings.TrimSpace(keyHash)
	if keyHash == "" {
		return nil, fmt.Errorf("key hash is required")
	}
	const query = `
SELECT k.id::text, k.user_id::text, k.domain_id::text, k.scopes, k.allowed_cidrs, k.permission_mode,
       CASE
         WHEN jsonb_typeof(domain_mcp.value->'allowed_scopes') = 'array' THEN ARRAY(SELECT jsonb_array_elements_text(domain_mcp.value->'allowed_scopes'))
         ELSE ARRAY[]::text[]
       END AS allowed_scopes
FROM user_mcp_access_keys k
LEFT JOIN runtime_config domain_mcp
  ON domain_mcp.scope_type = 'domain'
 AND domain_mcp.scope_id = k.domain_id
 AND domain_mcp.key = 'mcp.policy'
LEFT JOIN users u
  ON u.id = k.user_id
WHERE k.key_hash = $1
  AND k.revoked = false
  AND (k.expires_at IS NULL OR k.expires_at > now())
  AND COALESCE(domain_mcp.value->>'enabled', 'false') = 'true'
  AND COALESCE(domain_mcp.value->>'allow_user_access_keys', 'false') = 'true'
  AND (k.permission_mode <> 'bypass' OR COALESCE(domain_mcp.value->>'allow_bypass_mode', 'false') = 'true')
  AND COALESCE(u.settings->'webmail'->'mcp'->>'enabled', 'false') = 'true'
UNION ALL
SELECT id::text, '' AS user_id, domain_id::text, scopes, allowed_cidrs, 'basic' AS permission_mode, ARRAY[]::text[] AS allowed_scopes
FROM domain_api_keys
WHERE key_hash = $1
  AND revoked = false
  AND (expires_at IS NULL OR expires_at > now())
LIMIT 1`
	var info KeyInfo
	var cidrs, allowedScopes []string
	if err := v.db.QueryRowContext(ctx, query, keyHash).Scan(
		&info.ID,
		&info.UserID,
		&info.DomainID,
		pq.Array(&info.Scopes),
		pq.Array(&cidrs),
		&info.PermissionMode,
		pq.Array(&allowedScopes),
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("api key not found")
		}
		return nil, fmt.Errorf("verify api key: %w", err)
	}
	allowed, err := parseCIDRs(cidrs)
	if err != nil {
		return nil, err
	}
	if !CheckCIDR(ip, allowed) {
		return nil, fmt.Errorf("api key is not allowed from this client ip")
	}
	if strings.TrimSpace(info.UserID) != "" {
		if !scopesAllowedByDomainPolicy(info.Scopes, allowedScopes) {
			return nil, fmt.Errorf("api key scopes exceed domain mcp policy")
		}
		_, _ = v.db.ExecContext(ctx, `UPDATE user_mcp_access_keys SET last_used_at = now() WHERE id = $1::uuid`, info.ID)
	}
	return &info, nil
}

func scopesAllowedByDomainPolicy(scopes, allowedScopes []string) bool {
	allowed := make(map[string]struct{}, len(allowedScopes))
	for _, scope := range allowedScopes {
		scope = normalizeScope(scope)
		if scope != "" {
			allowed[scope] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return len(scopes) == 0
	}
	for _, scope := range scopes {
		scope = normalizeScope(scope)
		if scope == "" {
			continue
		}
		if _, ok := allowed[scope]; ok {
			continue
		}
		family, _, hasAction := strings.Cut(scope, ":")
		if !hasAction {
			if _, ok := allowed[family+":manage"]; ok {
				continue
			}
		}
		return false
	}
	return true
}

func normalizeScope(scope string) string {
	scope = strings.ToLower(strings.TrimSpace(scope))
	return strings.ReplaceAll(scope, ".", ":")
}

func parseCIDRs(values []string) ([]*net.IPNet, error) {
	if len(values) == 0 {
		return nil, nil
	}
	allowed := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if ip := net.ParseIP(value); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			value = fmt.Sprintf("%s/%d", ip.String(), bits)
		}
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("invalid api key cidr allowlist entry")
		}
		allowed = append(allowed, network)
	}
	return allowed, nil
}
