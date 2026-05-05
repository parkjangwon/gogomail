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
		return Principal{}, fmt.Errorf("principal kind %q is not implemented", req.Kind)
	}
}

func (r *Repository) ResolveAlias(ctx context.Context, req ResolveAliasRequest) (Alias, error) {
	if r == nil || r.db == nil {
		return Alias{}, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeResolveAliasRequest(req)
	if err != nil {
		return Alias{}, err
	}
	const query = `
SELECT a.id::text,
       a.company_id::text,
       a.domain_id::text,
       a.alias_address,
       a.alias_address_ace,
       a.target_kind,
       a.target_id::text,
       a.status
FROM directory_aliases a
JOIN domains d ON d.id = a.domain_id
JOIN companies c ON c.id = a.company_id AND c.id = d.company_id
WHERE lower(a.alias_address_ace) = $1
  AND ($2::boolean = false OR (a.status = 'active' AND d.status = 'active' AND c.status = 'active'))`
	var alias Alias
	if err := r.db.QueryRowContext(ctx, query, req.Address, req.ActiveOnly).Scan(
		&alias.ID,
		&alias.CompanyID,
		&alias.DomainID,
		&alias.Address,
		&alias.AddressACE,
		&alias.TargetKind,
		&alias.TargetID,
		&alias.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Alias{}, fmt.Errorf("directory alias not found")
		}
		return Alias{}, fmt.Errorf("resolve directory alias: %w", err)
	}
	target, err := r.ResolvePrincipal(ctx, ResolvePrincipalRequest{
		ID:         alias.TargetID,
		Kind:       alias.TargetKind,
		ActiveOnly: req.ActiveOnly,
	})
	if err != nil {
		return Alias{}, fmt.Errorf("resolve directory alias target: %w", err)
	}
	alias.TargetPrincipal = target
	return alias, nil
}

func (r *Repository) CheckDirectGroupMembership(ctx context.Context, req CheckGroupMembershipRequest) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeCheckGroupMembershipRequest(req)
	if err != nil {
		return false, err
	}
	const query = `
SELECT EXISTS (
  SELECT 1
  FROM directory_group_memberships m
  JOIN directory_groups g ON g.id = m.group_id
  JOIN domains d ON d.id = g.domain_id
  JOIN companies c ON c.id = g.company_id AND c.id = d.company_id
  WHERE m.group_id = $1::uuid
    AND m.member_kind = $2
    AND m.member_id = $3::uuid
    AND ($4::boolean = false OR (m.status = 'active' AND g.status = 'active' AND d.status = 'active' AND c.status = 'active'))
)`
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, req.GroupID, req.MemberKind, req.MemberID, req.ActiveOnly).Scan(&exists); err != nil {
		return false, fmt.Errorf("check direct group membership: %w", err)
	}
	return exists, nil
}

func (r *Repository) resolveUserPrincipal(ctx context.Context, req ResolvePrincipalRequest) (Principal, error) {
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

func (r *Repository) resolveOrganizationPrincipal(ctx context.Context, req ResolvePrincipalRequest) (Principal, error) {
	const query = `
SELECT o.id::text,
       c.id::text,
       d.id::text,
       o.id::text,
       o.name,
       '',
       o.status
FROM organizations o
JOIN domains d ON d.id = o.domain_id
JOIN companies c ON c.id = d.company_id
WHERE o.id = $1::uuid
  AND ($2::boolean = false OR (o.status = 'active' AND d.status = 'active' AND c.status = 'active'))`
	var principal Principal
	principal.Kind = PrincipalKindOrganization
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

func (r *Repository) resolveGroupPrincipal(ctx context.Context, req ResolvePrincipalRequest) (Principal, error) {
	const query = `
SELECT g.id::text,
       g.company_id::text,
       g.domain_id::text,
       COALESCE(g.org_id::text, ''),
       g.name,
       '',
       g.status
FROM directory_groups g
JOIN domains d ON d.id = g.domain_id
JOIN companies c ON c.id = g.company_id AND c.id = d.company_id
WHERE g.id = $1::uuid
  AND ($2::boolean = false OR (g.status = 'active' AND d.status = 'active' AND c.status = 'active'))`
	var principal Principal
	principal.Kind = PrincipalKindGroup
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

func (r *Repository) resolveResourcePrincipal(ctx context.Context, req ResolvePrincipalRequest) (Principal, error) {
	const query = `
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
JOIN companies c ON c.id = rsrc.company_id AND c.id = d.company_id
WHERE rsrc.id = $1::uuid
  AND ($2::boolean = false OR (rsrc.status = 'active' AND d.status = 'active' AND c.status = 'active'))`
	var principal Principal
	principal.Kind = PrincipalKindResource
	if err := r.db.QueryRowContext(ctx, query, req.ID, req.ActiveOnly).Scan(
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
			return Principal{}, fmt.Errorf("directory principal not found")
		}
		return Principal{}, fmt.Errorf("resolve directory principal: %w", err)
	}
	return principal, nil
}
