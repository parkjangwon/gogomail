package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/dkim"
)

type DKIMKey struct {
	ID            string
	DomainID      string
	DomainName    string
	Selector      string
	PrivateKeyPEM string
	PublicKeyDNS  string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type DKIMKeyView struct {
	ID           string    `json:"id"`
	DomainID     string    `json:"domain_id"`
	Selector     string    `json:"selector"`
	PublicKeyDNS string    `json:"public_key_dns"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateDKIMKeyInput struct {
	DomainID      string `json:"domain_id"`
	Selector      string `json:"selector"`
	PrivateKeyPEM string `json:"private_key_pem"`
	PublicKeyDNS  string `json:"public_key_dns"`
}

func (r *Repository) ActiveDKIMKey(ctx context.Context, domainID string) (DKIMKey, error) {
	if r.db == nil {
		return DKIMKey{}, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT
  id::text,
  domain_id::text,
  COALESCE(NULLIF(d.name_ace, ''), d.name),
  selector,
  private_key_pem,
  public_key_dns,
  status,
  created_at,
  updated_at
FROM dkim_keys
JOIN domains d ON d.id = dkim_keys.domain_id
WHERE dkim_keys.domain_id::text = $1
  AND dkim_keys.status = 'active'
ORDER BY updated_at DESC
LIMIT 1`

	var key DKIMKey
	if err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(domainID)).Scan(
		&key.ID,
		&key.DomainID,
		&key.DomainName,
		&key.Selector,
		&key.PrivateKeyPEM,
		&key.PublicKeyDNS,
		&key.Status,
		&key.CreatedAt,
		&key.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return DKIMKey{}, fmt.Errorf("active dkim key for domain %q not found", domainID)
		}
		return DKIMKey{}, fmt.Errorf("lookup active dkim key: %w", err)
	}
	return key, nil
}

func (r *Repository) ListDKIMKeys(ctx context.Context, domainID string, limit int) ([]DKIMKeyView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  id::text,
  domain_id::text,
  selector,
  public_key_dns,
  status,
  created_at,
  updated_at
FROM dkim_keys
WHERE ($1 = '' OR domain_id::text = $1)
ORDER BY updated_at DESC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(domainID), limit)
	if err != nil {
		return nil, fmt.Errorf("list dkim keys: %w", err)
	}
	defer rows.Close()

	var keys []DKIMKeyView
	for rows.Next() {
		var key DKIMKeyView
		if err := rows.Scan(
			&key.ID,
			&key.DomainID,
			&key.Selector,
			&key.PublicKeyDNS,
			&key.Status,
			&key.CreatedAt,
			&key.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dkim key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dkim keys: %w", err)
	}
	return keys, nil
}

func (r *Repository) CreateDKIMKey(ctx context.Context, input CreateDKIMKeyInput) (string, error) {
	if r.db == nil {
		return "", fmt.Errorf("database handle is required")
	}
	if strings.TrimSpace(input.DomainID) == "" {
		return "", fmt.Errorf("domain_id is required")
	}
	if strings.TrimSpace(input.Selector) == "" {
		return "", fmt.Errorf("selector is required")
	}
	if strings.TrimSpace(input.PrivateKeyPEM) == "" {
		return "", fmt.Errorf("private_key_pem is required")
	}
	publicKeyDNS := strings.TrimSpace(input.PublicKeyDNS)
	if publicKeyDNS == "" {
		derived, err := dkim.PublicKeyDNSFromPrivateKeyPEM(input.PrivateKeyPEM)
		if err != nil {
			return "", err
		}
		publicKeyDNS = derived
	}

	const query = `
INSERT INTO dkim_keys (domain_id, selector, private_key_pem, public_key_dns, status)
VALUES ($1, $2, $3, $4, 'active')
ON CONFLICT (domain_id, selector)
DO UPDATE SET
  private_key_pem = EXCLUDED.private_key_pem,
  public_key_dns = EXCLUDED.public_key_dns,
  status = 'active',
  updated_at = now()
RETURNING id::text`

	var id string
	if err := r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(input.DomainID),
		strings.TrimSpace(input.Selector),
		input.PrivateKeyPEM,
		publicKeyDNS,
	).Scan(&id); err != nil {
		return "", fmt.Errorf("create dkim key: %w", err)
	}
	return id, nil
}

func (r *Repository) DeactivateDKIMKey(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	const query = `
UPDATE dkim_keys
SET status = 'inactive',
    updated_at = now()
WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("deactivate dkim key: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("dkim key %q not found", id)
	}
	return nil
}
