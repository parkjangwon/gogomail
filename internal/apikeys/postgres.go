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
SELECT id::text, domain_id::text, scopes, allowed_cidrs
FROM domain_api_keys
WHERE key_hash = $1
  AND revoked = false
  AND (expires_at IS NULL OR expires_at > now())
LIMIT 1`
	var info KeyInfo
	var cidrs []string
	if err := v.db.QueryRowContext(ctx, query, keyHash).Scan(
		&info.ID,
		&info.DomainID,
		pq.Array(&info.Scopes),
		pq.Array(&cidrs),
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
	return &info, nil
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
