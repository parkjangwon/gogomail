package directory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ResolvePrincipal(ctx context.Context, req ResolvePrincipalRequest) (Principal, error) {
	if r == nil || r.db == nil {
		return Principal{}, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeResolvePrincipalRequest(req)
	if err != nil {
		return Principal{}, err
	}
	if req.Kind != PrincipalKindUser {
		return Principal{}, fmt.Errorf("principal kind %q is not implemented", req.Kind)
	}
	const query = `
SELECT u.id::text,
       c.id::text,
       d.id::text,
       COALESCE(u.org_id::text, ''),
       u.display_name,
       COALESCE(primary_addr.address, ''),
       u.status
FROM users u
JOIN domains d ON d.id = u.domain_id
JOIN companies c ON c.id = d.company_id
LEFT JOIN LATERAL (
  SELECT address
  FROM user_addresses
  WHERE user_id = u.id
  ORDER BY is_primary DESC, created_at ASC, id ASC
  LIMIT 1
) primary_addr ON TRUE
WHERE u.id = $1::uuid
  AND ($2::boolean = false OR (u.status = 'active' AND d.status = 'active' AND c.status = 'active'))`
	var principal Principal
	principal.Kind = PrincipalKindUser
	if err := r.db.QueryRowContext(ctx, query, req.ID, req.ActiveOnly).Scan(
		&principal.ID,
		&principal.CompanyID,
		&principal.DomainID,
		&principal.OrganizationID,
		&principal.DisplayName,
		&principal.PrimaryEmail,
		&principal.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Principal{}, fmt.Errorf("directory principal not found")
		}
		return Principal{}, fmt.Errorf("resolve directory principal: %w", err)
	}
	return principal, nil
}
