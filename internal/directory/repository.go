package directory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

type Repository struct {
	db *sql.DB
}

type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

var ErrPrincipalNotFound = errors.New("directory principal not found")
var ErrAliasAlreadyExists = errors.New("directory alias already exists")
var ErrDelegationAlreadyExists = errors.New("directory delegation already exists")
var ErrGroupMembershipAlreadyExists = errors.New("directory group membership already exists")

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
	switch req.Kind {
	case PrincipalKindUser:
		return r.resolveUserPrincipal(ctx, req)
	case PrincipalKindOrganization:
		return r.resolveOrganizationPrincipal(ctx, req)
	case PrincipalKindGroup:
		return r.resolveGroupPrincipal(ctx, req)
	case PrincipalKindResource:
		return r.resolveResourcePrincipal(ctx, req)
	default:
		return Principal{}, fmt.Errorf("unsupported principal kind %q", req.Kind)
	}
}

const resolveUserByEmailBaseSQL = `
SELECT u.id::text,
       c.id::text,
       d.id::text,
       COALESCE(u.org_id::text, ''),
       u.display_name,
       COALESCE(a.address, ''),
       u.status
FROM users u
JOIN domains d ON d.id = u.domain_id
JOIN companies c ON c.id = d.company_id
JOIN user_addresses a ON a.user_id = u.id AND a.address_ace = $1`

func buildResolveUserByEmailQuery(req ResolveUserByEmailRequest) (string, []any) {
	conditions := []string{}
	if req.ActiveOnly {
		conditions = append(conditions, "(u.status = 'active' AND d.status = 'active' AND c.status = 'active')")
	}
	query := resolveUserByEmailBaseSQL
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	return query, []any{strings.ToLower(strings.TrimSpace(req.Email))}
}

func (r *Repository) ResolveUserByEmail(ctx context.Context, req ResolveUserByEmailRequest) (Principal, error) {
	if r == nil || r.db == nil {
		return Principal{}, fmt.Errorf("database handle is required")
	}
	if req.Email == "" {
		return Principal{}, fmt.Errorf("email is required")
	}
	query, args := buildResolveUserByEmailQuery(req)
	var principal Principal
	principal.Kind = PrincipalKindUser
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&principal.ID,
		&principal.CompanyID,
		&principal.DomainID,
		&principal.OrganizationID,
		&principal.DisplayName,
		&principal.PrimaryEmail,
		&principal.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Principal{}, ErrPrincipalNotFound
		}
		return Principal{}, fmt.Errorf("resolve user by email: %w", err)
	}
	return principal, nil
}

const resolveUsersByEmailsBaseSQL = `
SELECT req.email,
       u.id::text,
       c.id::text,
       d.id::text,
       COALESCE(u.org_id::text, ''),
       u.display_name,
       COALESCE(primary_addr.address, ''),
       u.status
FROM unnest($1::text[]) WITH ORDINALITY AS req(email, email_order)
JOIN user_addresses lookup ON lookup.address_ace = req.email
JOIN users u ON u.id = lookup.user_id
JOIN domains d ON d.id = u.domain_id
JOIN companies c ON c.id = d.company_id
LEFT JOIN user_addresses primary_addr ON primary_addr.user_id = u.id AND primary_addr.is_primary = true`

func buildResolveUsersByEmailsQuery(activeOnly bool) string {
	query := resolveUsersByEmailsBaseSQL
	if activeOnly {
		query += "\nWHERE (u.status = 'active' AND d.status = 'active' AND c.status = 'active')"
	}
	query += "\nORDER BY req.email_order"
	return query
}

func (r *Repository) ResolveUsersByEmails(ctx context.Context, emails []string, activeOnly bool) (map[string]Principal, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	emails = normalizeEmailList(emails)
	out := make(map[string]Principal, len(emails))
	if len(emails) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, buildResolveUsersByEmailsQuery(activeOnly), pq.Array(emails))
	if err != nil {
		return nil, fmt.Errorf("resolve users by emails: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var email string
		principal := Principal{Kind: PrincipalKindUser}
		if err := rows.Scan(
			&email,
			&principal.ID,
			&principal.CompanyID,
			&principal.DomainID,
			&principal.OrganizationID,
			&principal.DisplayName,
			&principal.PrimaryEmail,
			&principal.Status,
		); err != nil {
			return nil, fmt.Errorf("scan resolved user by email: %w", err)
		}
		key := strings.ToLower(strings.TrimSpace(email))
		if _, exists := out[key]; !exists {
			out[key] = principal
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resolve users by emails rows: %w", err)
	}
	return out, nil
}

func normalizeEmailList(emails []string) []string {
	out := make([]string, 0, len(emails))
	seen := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		out = append(out, email)
	}
	return out
}

const resolveUserPrincipalBaseSQL = `
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
) primary_addr ON TRUE`

func buildResolveUserPrincipalQuery(req ResolvePrincipalRequest) (string, []any) {
	conditions := []string{"u.id = $1::uuid"}
	if req.ActiveOnly {
		conditions = append(conditions, "(u.status = 'active' AND d.status = 'active' AND c.status = 'active')")
	}
	return resolveUserPrincipalBaseSQL + "\nWHERE " + strings.Join(conditions, "\n  AND "), []any{req.ID}
}

func (r *Repository) resolveUserPrincipal(ctx context.Context, req ResolvePrincipalRequest) (Principal, error) {
	query, args := buildResolveUserPrincipalQuery(req)
	var principal Principal
	principal.Kind = PrincipalKindUser
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&principal.ID,
		&principal.CompanyID,
		&principal.DomainID,
		&principal.OrganizationID,
		&principal.DisplayName,
		&principal.PrimaryEmail,
		&principal.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Principal{}, ErrPrincipalNotFound
		}
		return Principal{}, fmt.Errorf("resolve directory principal: %w", err)
	}
	return principal, nil
}

const resolveOrganizationPrincipalBaseSQL = `
SELECT o.id::text,
       c.id::text,
       d.id::text,
       o.id::text,
       o.name,
       '',
       o.status
FROM organizations o
JOIN domains d ON d.id = o.domain_id
JOIN companies c ON c.id = d.company_id`

func buildResolveOrganizationPrincipalQuery(req ResolvePrincipalRequest) (string, []any) {
	conditions := []string{"o.id = $1::uuid"}
	if req.ActiveOnly {
		conditions = append(conditions, "(o.status = 'active' AND d.status = 'active' AND c.status = 'active')")
	}
	return resolveOrganizationPrincipalBaseSQL + "\nWHERE " + strings.Join(conditions, "\n  AND "), []any{req.ID}
}

func (r *Repository) resolveOrganizationPrincipal(ctx context.Context, req ResolvePrincipalRequest) (Principal, error) {
	query, args := buildResolveOrganizationPrincipalQuery(req)
	var principal Principal
	principal.Kind = PrincipalKindOrganization
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&principal.ID,
		&principal.CompanyID,
		&principal.DomainID,
		&principal.OrganizationID,
		&principal.DisplayName,
		&principal.PrimaryEmail,
		&principal.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Principal{}, ErrPrincipalNotFound
		}
		return Principal{}, fmt.Errorf("resolve directory principal: %w", err)
	}
	return principal, nil
}

const resolveGroupPrincipalBaseSQL = `
SELECT g.id::text,
       g.company_id::text,
       g.domain_id::text,
       COALESCE(g.org_id::text, ''),
       g.name,
       '',
       g.status
FROM directory_groups g
JOIN domains d ON d.id = g.domain_id
JOIN companies c ON c.id = g.company_id AND c.id = d.company_id`

func buildResolveGroupPrincipalQuery(req ResolvePrincipalRequest) (string, []any) {
	conditions := []string{"g.id = $1::uuid"}
	if req.ActiveOnly {
		conditions = append(conditions, "(g.status = 'active' AND d.status = 'active' AND c.status = 'active')")
	}
	return resolveGroupPrincipalBaseSQL + "\nWHERE " + strings.Join(conditions, "\n  AND "), []any{req.ID}
}

func (r *Repository) resolveGroupPrincipal(ctx context.Context, req ResolvePrincipalRequest) (Principal, error) {
	query, args := buildResolveGroupPrincipalQuery(req)
	var principal Principal
	principal.Kind = PrincipalKindGroup
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&principal.ID,
		&principal.CompanyID,
		&principal.DomainID,
		&principal.OrganizationID,
		&principal.DisplayName,
		&principal.PrimaryEmail,
		&principal.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Principal{}, ErrPrincipalNotFound
		}
		return Principal{}, fmt.Errorf("resolve directory principal: %w", err)
	}
	return principal, nil
}

const resolveResourcePrincipalBaseSQL = `
SELECT rsrc.id::text,
       rsrc.company_id::text,
       rsrc.domain_id::text,
       COALESCE(rsrc.org_id::text, ''),
       rsrc.name,
       '',
       rsrc.status,
       rsrc.resource_type
FROM directory_resources rsrc
JOIN domains d ON d.id = rsrc.domain_id
JOIN companies c ON c.id = rsrc.company_id AND c.id = d.company_id`

func buildResolveResourcePrincipalQuery(req ResolvePrincipalRequest) (string, []any) {
	conditions := []string{"rsrc.id = $1::uuid"}
	if req.ActiveOnly {
		conditions = append(conditions, "(rsrc.status = 'active' AND d.status = 'active' AND c.status = 'active')")
	}
	return resolveResourcePrincipalBaseSQL + "\nWHERE " + strings.Join(conditions, "\n  AND "), []any{req.ID}
}

func (r *Repository) resolveResourcePrincipal(ctx context.Context, req ResolvePrincipalRequest) (Principal, error) {
	query, args := buildResolveResourcePrincipalQuery(req)
	var principal Principal
	principal.Kind = PrincipalKindResource
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&principal.ID,
		&principal.CompanyID,
		&principal.DomainID,
		&principal.OrganizationID,
		&principal.DisplayName,
		&principal.PrimaryEmail,
		&principal.Status,
		&principal.ResourceType,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Principal{}, ErrPrincipalNotFound
		}
		return Principal{}, fmt.Errorf("resolve directory principal: %w", err)
	}
	return principal, nil
}
