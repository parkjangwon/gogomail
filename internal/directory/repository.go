package directory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/jackc/pgx/v5/pgconn"
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

func (r *Repository) CreateAlias(ctx context.Context, req CreateAliasRequest) (Alias, error) {
	if r == nil || r.db == nil {
		return Alias{}, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeCreateAliasRequest(req)
	if err != nil {
		return Alias{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Alias{}, fmt.Errorf("begin create directory alias transaction: %w", err)
	}
	defer tx.Rollback()
	alias, err := r.createAliasTx(ctx, tx, req)
	if err != nil {
		return Alias{}, err
	}
	if err := tx.Commit(); err != nil {
		return Alias{}, fmt.Errorf("commit create directory alias transaction: %w", err)
	}
	return alias, nil
}

func (r *Repository) CreateAliasWithAudit(ctx context.Context, req CreateAliasRequest) (Alias, error) {
	if r == nil || r.db == nil {
		return Alias{}, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeCreateAliasRequest(req)
	if err != nil {
		return Alias{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Alias{}, fmt.Errorf("begin create directory alias transaction: %w", err)
	}
	defer tx.Rollback()
	alias, err := r.createAliasTx(ctx, tx, req)
	if err != nil {
		return Alias{}, err
	}
	detail, err := directoryAliasCreateAuditDetail(alias)
	if err != nil {
		return Alias{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  alias.CompanyID,
		DomainID:   alias.DomainID,
		Category:   "admin",
		Action:     "directory_alias.create",
		TargetType: "directory_alias",
		TargetID:   alias.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return Alias{}, fmt.Errorf("record directory alias create audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Alias{}, fmt.Errorf("commit create directory alias transaction: %w", err)
	}
	return alias, nil
}

func (r *Repository) createAliasTx(ctx context.Context, tx *sql.Tx, req CreateAliasRequest) (Alias, error) {
	domainName, err := activeDomainNameACE(ctx, tx, req.CompanyID, req.DomainID)
	if err != nil {
		return Alias{}, err
	}
	if !aliasAddressMatchesDomain(req.Address, domainName) {
		return Alias{}, fmt.Errorf("alias address domain does not match directory domain")
	}
	target, err := r.ResolvePrincipal(ctx, ResolvePrincipalRequest{
		ID:         req.TargetID,
		Kind:       req.TargetKind,
		ActiveOnly: true,
	})
	if err != nil {
		return Alias{}, fmt.Errorf("resolve directory alias target: %w", err)
	}
	if target.CompanyID != req.CompanyID {
		return Alias{}, fmt.Errorf("directory alias target belongs to a different company")
	}
	const query = `
INSERT INTO directory_aliases (
  company_id,
  domain_id,
  alias_address,
  alias_address_ace,
  target_kind,
  target_id
)
VALUES ($1::uuid, $2::uuid, $3, $3, $4, $5::uuid)
RETURNING id::text,
          company_id::text,
          domain_id::text,
          alias_address,
          alias_address_ace,
          target_kind,
          target_id::text,
          status`
	var alias Alias
	if err := tx.QueryRowContext(ctx, query,
		req.CompanyID,
		req.DomainID,
		req.Address,
		req.TargetKind,
		req.TargetID,
	).Scan(
		&alias.ID,
		&alias.CompanyID,
		&alias.DomainID,
		&alias.Address,
		&alias.AddressACE,
		&alias.TargetKind,
		&alias.TargetID,
		&alias.Status,
	); err != nil {
		return Alias{}, mapDirectoryAliasInsertError(err)
	}
	alias.TargetPrincipal = target
	return alias, nil
}

func directoryAliasCreateAuditDetail(alias Alias) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"alias_id":    alias.ID,
		"company_id":  alias.CompanyID,
		"domain_id":   alias.DomainID,
		"address":     alias.Address,
		"address_ace": alias.AddressACE,
		"target_kind": alias.TargetKind,
		"target_id":   alias.TargetID,
		"status":      alias.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal directory alias create audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) DeleteAliasWithAudit(ctx context.Context, id string) (Alias, error) {
	if r == nil || r.db == nil {
		return Alias{}, fmt.Errorf("database handle is required")
	}
	id, err := NormalizePrincipalID(id)
	if err != nil {
		return Alias{}, fmt.Errorf("alias id: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Alias{}, fmt.Errorf("begin delete directory alias transaction: %w", err)
	}
	defer tx.Rollback()
	alias, err := r.deleteAliasTx(ctx, tx, id)
	if err != nil {
		return Alias{}, err
	}
	detail, err := directoryAliasDeleteAuditDetail(alias)
	if err != nil {
		return Alias{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  alias.CompanyID,
		DomainID:   alias.DomainID,
		Category:   "admin",
		Action:     "directory_alias.delete",
		TargetType: "directory_alias",
		TargetID:   alias.ID,
		Result:     "deleted",
		Detail:     detail,
	}); err != nil {
		return Alias{}, fmt.Errorf("record directory alias delete audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Alias{}, fmt.Errorf("commit delete directory alias transaction: %w", err)
	}
	return alias, nil
}

func (r *Repository) deleteAliasTx(ctx context.Context, tx *sql.Tx, id string) (Alias, error) {
	const query = `
UPDATE directory_aliases
SET status = 'deleted',
    updated_at = now()
WHERE id = $1::uuid
  AND status = 'active'
RETURNING id::text,
          company_id::text,
          domain_id::text,
          alias_address,
          alias_address_ace,
          target_kind,
          target_id::text,
          status`
	var alias Alias
	if err := tx.QueryRowContext(ctx, query, id).Scan(
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
		return Alias{}, fmt.Errorf("delete directory alias: %w", err)
	}
	target, err := r.ResolvePrincipal(ctx, ResolvePrincipalRequest{
		ID:         alias.TargetID,
		Kind:       alias.TargetKind,
		ActiveOnly: false,
	})
	if err != nil {
		return Alias{}, fmt.Errorf("resolve deleted directory alias target: %w", err)
	}
	alias.TargetPrincipal = target
	return alias, nil
}

func directoryAliasDeleteAuditDetail(alias Alias) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"alias_id":        alias.ID,
		"company_id":      alias.CompanyID,
		"domain_id":       alias.DomainID,
		"address":         alias.Address,
		"address_ace":     alias.AddressACE,
		"target_kind":     alias.TargetKind,
		"target_id":       alias.TargetID,
		"previous_status": "active",
		"status":          alias.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal directory alias delete audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ListAliases(ctx context.Context, req ListAliasesRequest) ([]Alias, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeListAliasesRequest(req)
	if err != nil {
		return nil, err
	}
	pattern := principalSearchPattern(req.Query)
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
WHERE a.company_id = $1::uuid
  AND ($2 = '' OR a.domain_id = NULLIF($2, '')::uuid)
  AND ($3 = '' OR a.target_kind = $3)
  AND ($4 = '' OR a.target_id = NULLIF($4, '')::uuid)
  AND ($5 = '' OR lower(a.alias_address) LIKE $5 ESCAPE '\' OR lower(a.alias_address_ace) LIKE $5 ESCAPE '\')
  AND ($6::boolean = false OR (a.status = 'active' AND d.status = 'active' AND c.status = 'active'))
ORDER BY lower(a.alias_address_ace), a.id
LIMIT $7`
	rows, err := r.db.QueryContext(ctx, query,
		req.CompanyID,
		req.DomainID,
		req.TargetKind,
		req.TargetID,
		pattern,
		req.ActiveOnly,
		req.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list directory aliases: %w", err)
	}
	defer rows.Close()
	aliases := make([]Alias, 0, req.Limit)
	for rows.Next() {
		var alias Alias
		if err := rows.Scan(
			&alias.ID,
			&alias.CompanyID,
			&alias.DomainID,
			&alias.Address,
			&alias.AddressACE,
			&alias.TargetKind,
			&alias.TargetID,
			&alias.Status,
		); err != nil {
			return nil, fmt.Errorf("scan directory alias list result: %w", err)
		}
		alias.TargetPrincipal, err = r.ResolvePrincipal(ctx, ResolvePrincipalRequest{
			ID:         alias.TargetID,
			Kind:       alias.TargetKind,
			ActiveOnly: req.ActiveOnly,
		})
		if err != nil {
			return nil, fmt.Errorf("resolve directory alias list target: %w", err)
		}
		aliases = append(aliases, alias)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list directory alias rows: %w", err)
	}
	return aliases, nil
}

func (r *Repository) activeDomainNameACE(ctx context.Context, companyID string, domainID string) (string, error) {
	return activeDomainNameACE(ctx, r.db, companyID, domainID)
}

func activeDomainNameACE(ctx context.Context, q rowQuerier, companyID string, domainID string) (string, error) {
	const query = `
SELECT d.name_ace
FROM domains d
JOIN companies c ON c.id = d.company_id
WHERE c.id = $1::uuid
  AND d.id = $2::uuid
  AND c.status = 'active'
  AND d.status = 'active'`
	var name string
	if err := q.QueryRowContext(ctx, query, companyID, domainID).Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("directory domain not found")
		}
		return "", fmt.Errorf("read directory domain: %w", err)
	}
	return strings.ToLower(strings.TrimSpace(name)), nil
}

func aliasAddressMatchesDomain(address string, domain string) bool {
	_, addressDomain, ok := strings.Cut(address, "@")
	return ok && strings.EqualFold(addressDomain, domain)
}

func mapDirectoryAliasInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "idx_directory_aliases_active_address":
			return fmt.Errorf("%w", ErrAliasAlreadyExists)
		}
	}
	return fmt.Errorf("create directory alias: %w", err)
}

func (r *Repository) SearchPrincipals(ctx context.Context, req SearchPrincipalsRequest) ([]Principal, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeSearchPrincipalsRequest(req)
	if err != nil {
		return nil, err
	}
	allowUser := searchPrincipalKindAllowed(req.Kinds, PrincipalKindUser)
	allowOrganization := searchPrincipalKindAllowed(req.Kinds, PrincipalKindOrganization)
	allowGroup := searchPrincipalKindAllowed(req.Kinds, PrincipalKindGroup)
	allowResource := searchPrincipalKindAllowed(req.Kinds, PrincipalKindResource)
	pattern := principalSearchPattern(req.Query)
	const query = `
WITH principals AS (
  SELECT u.id::text AS id,
         'user' AS kind,
         c.id::text AS company_id,
         d.id::text AS domain_id,
         COALESCE(u.org_id::text, '') AS organization_id,
         u.display_name AS display_name,
         COALESCE(primary_addr.address, '') AS primary_email,
         u.status AS status,
         '' AS resource_type,
         1 AS kind_rank
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
  WHERE $7::boolean
    AND c.id = $1::uuid
    AND ($2 = '' OR d.id = NULLIF($2, '')::uuid)
    AND ($3 = '' OR COALESCE(u.org_id::text, '') = $3)
    AND ($4::boolean = false OR (u.status = 'active' AND d.status = 'active' AND c.status = 'active'))
    AND ($5 = '' OR lower(u.display_name) LIKE $5 ESCAPE '\' OR lower(u.username) LIKE $5 ESCAPE '\' OR lower(COALESCE(primary_addr.address, '')) LIKE $5 ESCAPE '\')
  UNION ALL
  SELECT o.id::text,
         'organization',
         c.id::text,
         d.id::text,
         o.id::text,
         o.name,
         '',
         o.status,
         '',
         2
  FROM organizations o
  JOIN domains d ON d.id = o.domain_id
  JOIN companies c ON c.id = d.company_id
  WHERE $8::boolean
    AND c.id = $1::uuid
    AND ($2 = '' OR d.id = NULLIF($2, '')::uuid)
    AND ($3 = '' OR o.id::text = $3)
    AND ($4::boolean = false OR (o.status = 'active' AND d.status = 'active' AND c.status = 'active'))
    AND ($5 = '' OR lower(o.name) LIKE $5 ESCAPE '\' OR lower(o.code) LIKE $5 ESCAPE '\')
  UNION ALL
  SELECT g.id::text,
         'group',
         g.company_id::text,
         g.domain_id::text,
         COALESCE(g.org_id::text, ''),
         g.name,
         '',
         g.status,
         '',
         3
  FROM directory_groups g
  JOIN domains d ON d.id = g.domain_id
  JOIN companies c ON c.id = g.company_id AND c.id = d.company_id
  WHERE $9::boolean
    AND g.company_id = $1::uuid
    AND ($2 = '' OR g.domain_id = NULLIF($2, '')::uuid)
    AND ($3 = '' OR COALESCE(g.org_id::text, '') = $3)
    AND ($4::boolean = false OR (g.status = 'active' AND d.status = 'active' AND c.status = 'active'))
    AND ($5 = '' OR lower(g.name) LIKE $5 ESCAPE '\' OR lower(g.slug) LIKE $5 ESCAPE '\')
  UNION ALL
  SELECT rsrc.id::text,
         'resource',
         rsrc.company_id::text,
         rsrc.domain_id::text,
         COALESCE(rsrc.org_id::text, ''),
         rsrc.name,
         '',
         rsrc.status,
         rsrc.resource_type,
         4
  FROM directory_resources rsrc
  JOIN domains d ON d.id = rsrc.domain_id
  JOIN companies c ON c.id = rsrc.company_id AND c.id = d.company_id
  WHERE $10::boolean
    AND rsrc.company_id = $1::uuid
    AND ($2 = '' OR rsrc.domain_id = NULLIF($2, '')::uuid)
    AND ($3 = '' OR COALESCE(rsrc.org_id::text, '') = $3)
    AND ($4::boolean = false OR (rsrc.status = 'active' AND d.status = 'active' AND c.status = 'active'))
    AND ($5 = '' OR lower(rsrc.name) LIKE $5 ESCAPE '\' OR lower(rsrc.slug) LIKE $5 ESCAPE '\' OR lower(rsrc.resource_type) LIKE $5 ESCAPE '\')
)
SELECT id, kind, company_id, domain_id, organization_id, display_name, primary_email, status, resource_type
FROM principals
ORDER BY kind_rank, lower(display_name), id
LIMIT $6`
	rows, err := r.db.QueryContext(ctx, query,
		req.CompanyID,
		req.DomainID,
		req.OrganizationID,
		req.ActiveOnly,
		pattern,
		req.Limit,
		allowUser,
		allowOrganization,
		allowGroup,
		allowResource,
	)
	if err != nil {
		return nil, fmt.Errorf("search directory principals: %w", err)
	}
	defer rows.Close()
	principals := make([]Principal, 0, req.Limit)
	for rows.Next() {
		var principal Principal
		if err := rows.Scan(
			&principal.ID,
			&principal.Kind,
			&principal.CompanyID,
			&principal.DomainID,
			&principal.OrganizationID,
			&principal.DisplayName,
			&principal.PrimaryEmail,
			&principal.Status,
			&principal.ResourceType,
		); err != nil {
			return nil, fmt.Errorf("scan directory principal search result: %w", err)
		}
		principals = append(principals, principal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search directory principal rows: %w", err)
	}
	return principals, nil
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

func (r *Repository) CreateGroupMembershipWithAudit(ctx context.Context, req CreateGroupMembershipRequest) (GroupMembership, error) {
	if r == nil || r.db == nil {
		return GroupMembership{}, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeCreateGroupMembershipRequest(req)
	if err != nil {
		return GroupMembership{}, err
	}
	companyID, err := r.ensureGroupMembershipPrincipalsActive(ctx, req)
	if err != nil {
		return GroupMembership{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return GroupMembership{}, fmt.Errorf("begin create directory group membership transaction: %w", err)
	}
	defer tx.Rollback()
	membership, err := r.createGroupMembershipTx(ctx, tx, req, companyID)
	if err != nil {
		return GroupMembership{}, err
	}
	detail, err := directoryGroupMembershipCreateAuditDetail(membership)
	if err != nil {
		return GroupMembership{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  membership.CompanyID,
		Category:   "admin",
		Action:     "directory_group_membership.create",
		TargetType: "directory_group_membership",
		TargetID:   membership.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return GroupMembership{}, fmt.Errorf("record directory group membership create audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return GroupMembership{}, fmt.Errorf("commit create directory group membership transaction: %w", err)
	}
	return membership, nil
}

func (r *Repository) CheckEffectiveGroupMembership(ctx context.Context, req CheckGroupMembershipRequest) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeCheckGroupMembershipRequest(req)
	if err != nil {
		return false, err
	}
	const query = `
WITH RECURSIVE group_tree(group_id, depth, path) AS (
  SELECT $1::uuid, 0, ARRAY[$1::uuid]
  UNION ALL
  SELECT m.member_id, group_tree.depth + 1, group_tree.path || m.member_id
  FROM group_tree
  JOIN directory_group_memberships m ON m.group_id = group_tree.group_id
  JOIN directory_groups nested_group ON nested_group.id = m.member_id
  JOIN domains d ON d.id = nested_group.domain_id
  JOIN companies c ON c.id = nested_group.company_id AND c.id = d.company_id
  WHERE m.member_kind = 'group'
    AND group_tree.depth < $5
    AND NOT m.member_id = ANY(group_tree.path)
    AND ($4::boolean = false OR (m.status = 'active' AND nested_group.status = 'active' AND d.status = 'active' AND c.status = 'active'))
)
SELECT EXISTS (
  SELECT 1
  FROM group_tree
  JOIN directory_group_memberships m ON m.group_id = group_tree.group_id
  JOIN directory_groups owning_group ON owning_group.id = m.group_id
  JOIN domains d ON d.id = owning_group.domain_id
  JOIN companies c ON c.id = owning_group.company_id AND c.id = d.company_id
  WHERE m.member_kind = $2
    AND m.member_id = $3::uuid
    AND ($4::boolean = false OR (m.status = 'active' AND owning_group.status = 'active' AND d.status = 'active' AND c.status = 'active'))
)`
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, req.GroupID, req.MemberKind, req.MemberID, req.ActiveOnly, req.MaxDepth).Scan(&exists); err != nil {
		return false, fmt.Errorf("check effective group membership: %w", err)
	}
	return exists, nil
}

func (r *Repository) createGroupMembershipTx(ctx context.Context, tx *sql.Tx, req CreateGroupMembershipRequest, companyID string) (GroupMembership, error) {
	const query = `
INSERT INTO directory_group_memberships (
  group_id,
  member_kind,
  member_id,
  role
)
VALUES ($1::uuid, $2, $3::uuid, $4)
RETURNING id::text,
          group_id::text,
          member_kind,
          member_id::text,
          role,
          status`
	var membership GroupMembership
	if err := tx.QueryRowContext(ctx, query,
		req.GroupID,
		req.MemberKind,
		req.MemberID,
		req.Role,
	).Scan(
		&membership.ID,
		&membership.GroupID,
		&membership.MemberKind,
		&membership.MemberID,
		&membership.Role,
		&membership.Status,
	); err != nil {
		return GroupMembership{}, mapDirectoryGroupMembershipInsertError(err)
	}
	membership.CompanyID = companyID
	return membership, nil
}

func directoryGroupMembershipCreateAuditDetail(membership GroupMembership) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"membership_id": membership.ID,
		"group_id":      membership.GroupID,
		"company_id":    membership.CompanyID,
		"member_kind":   membership.MemberKind,
		"member_id":     membership.MemberID,
		"role":          membership.Role,
		"status":        membership.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal directory group membership create audit detail: %w", err)
	}
	return detail, nil
}

func mapDirectoryGroupMembershipInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "idx_directory_group_memberships_active_member":
			return fmt.Errorf("%w", ErrGroupMembershipAlreadyExists)
		}
	}
	return fmt.Errorf("create directory group membership: %w", err)
}

func (r *Repository) ensureGroupMembershipPrincipalsActive(ctx context.Context, req CreateGroupMembershipRequest) (string, error) {
	group, err := r.ResolvePrincipal(ctx, ResolvePrincipalRequest{
		ID:         req.GroupID,
		Kind:       PrincipalKindGroup,
		ActiveOnly: true,
	})
	if err != nil {
		return "", fmt.Errorf("resolve active membership group: %w", err)
	}
	member, err := r.ResolvePrincipal(ctx, ResolvePrincipalRequest{
		ID:         req.MemberID,
		Kind:       req.MemberKind,
		ActiveOnly: true,
	})
	if err != nil {
		return "", fmt.Errorf("resolve active membership member: %w", err)
	}
	if group.CompanyID != member.CompanyID {
		return "", fmt.Errorf("directory group membership principals must belong to the same company")
	}
	if req.MemberKind == PrincipalKindGroup {
		wouldCycle, err := r.CheckEffectiveGroupMembership(ctx, CheckGroupMembershipRequest{
			GroupID:    req.MemberID,
			MemberKind: PrincipalKindGroup,
			MemberID:   req.GroupID,
			ActiveOnly: true,
			MaxDepth:   MaxGroupMembershipDepth,
		})
		if err != nil {
			return "", fmt.Errorf("check directory group membership cycle: %w", err)
		}
		if wouldCycle {
			return "", fmt.Errorf("directory group membership would create a cycle")
		}
	}
	return group.CompanyID, nil
}

func (r *Repository) CheckDelegation(ctx context.Context, req CheckDelegationRequest) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeCheckDelegationRequest(req)
	if err != nil {
		return false, err
	}
	const query = `
SELECT d.role
FROM directory_delegations d
JOIN companies c ON c.id = d.company_id
WHERE d.company_id = $1::uuid
  AND d.owner_kind = $2
  AND d.owner_id = $3::uuid
  AND d.delegate_kind = $4
  AND d.delegate_id = $5::uuid
  AND d.scope = $6
  AND ($7::boolean = false OR (d.status = 'active' AND c.status = 'active'))`
	rows, err := r.db.QueryContext(ctx, query,
		req.CompanyID,
		req.OwnerKind,
		req.OwnerID,
		req.DelegateKind,
		req.DelegateID,
		req.Scope,
		req.ActiveOnly,
	)
	if err != nil {
		return false, fmt.Errorf("check directory delegation: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return false, fmt.Errorf("scan directory delegation: %w", err)
		}
		if DelegationRoleSatisfies(role, req.RequiredRole) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("check directory delegation rows: %w", err)
	}
	return false, nil
}

func (r *Repository) CreateDelegationWithAudit(ctx context.Context, req CreateDelegationRequest) (Delegation, error) {
	if r == nil || r.db == nil {
		return Delegation{}, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeCreateDelegationRequest(req)
	if err != nil {
		return Delegation{}, err
	}
	if err := r.ensureDelegationPrincipalsActive(ctx, req); err != nil {
		return Delegation{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Delegation{}, fmt.Errorf("begin create directory delegation transaction: %w", err)
	}
	defer tx.Rollback()
	delegation, err := r.createDelegationTx(ctx, tx, req)
	if err != nil {
		return Delegation{}, err
	}
	detail, err := directoryDelegationCreateAuditDetail(delegation)
	if err != nil {
		return Delegation{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  delegation.CompanyID,
		Category:   "admin",
		Action:     "directory_delegation.create",
		TargetType: "directory_delegation",
		TargetID:   delegation.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return Delegation{}, fmt.Errorf("record directory delegation create audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Delegation{}, fmt.Errorf("commit create directory delegation transaction: %w", err)
	}
	return delegation, nil
}

func (r *Repository) CheckEffectiveDelegation(ctx context.Context, req CheckDelegationRequest) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeCheckDelegationRequest(req)
	if err != nil {
		return false, err
	}
	if req.ActiveOnly {
		ok, err := r.checkDelegationPrincipalsActive(ctx, req)
		if err != nil || !ok {
			return false, err
		}
	}
	const query = `
WITH RECURSIVE delegated_groups(root_group_id, group_id, depth, path, role) AS (
  SELECT d.delegate_id,
         d.delegate_id,
         0,
         ARRAY[d.delegate_id],
         d.role
  FROM directory_delegations d
  JOIN companies c ON c.id = d.company_id
  JOIN directory_groups root_group ON root_group.id = d.delegate_id AND root_group.company_id = d.company_id
  JOIN domains root_domain ON root_domain.id = root_group.domain_id AND root_domain.company_id = root_group.company_id
  WHERE d.company_id = $1::uuid
    AND d.owner_kind = $2
    AND d.owner_id = $3::uuid
    AND d.delegate_kind = 'group'
    AND d.scope = $6
    AND ($7::boolean = false OR (
      d.status = 'active' AND c.status = 'active' AND root_group.status = 'active' AND root_domain.status = 'active'
    ))
  UNION ALL
  SELECT delegated_groups.root_group_id,
         m.member_id,
         delegated_groups.depth + 1,
         delegated_groups.path || m.member_id,
         delegated_groups.role
  FROM delegated_groups
  JOIN directory_group_memberships m ON m.group_id = delegated_groups.group_id
  JOIN directory_groups nested_group ON nested_group.id = m.member_id
  JOIN domains nested_domain ON nested_domain.id = nested_group.domain_id AND nested_domain.company_id = nested_group.company_id
  JOIN companies nested_company ON nested_company.id = nested_group.company_id AND nested_company.id = $1::uuid
  WHERE m.member_kind = 'group'
    AND delegated_groups.depth < $8
    AND NOT m.member_id = ANY(delegated_groups.path)
    AND ($7::boolean = false OR (
      m.status = 'active' AND nested_group.status = 'active' AND nested_domain.status = 'active' AND nested_company.status = 'active'
    ))
),
candidate_roles AS (
  SELECT d.role
  FROM directory_delegations d
  JOIN companies c ON c.id = d.company_id
  WHERE d.company_id = $1::uuid
    AND d.owner_kind = $2
    AND d.owner_id = $3::uuid
    AND d.delegate_kind = $4
    AND d.delegate_id = $5::uuid
    AND d.scope = $6
    AND ($7::boolean = false OR (d.status = 'active' AND c.status = 'active'))
  UNION ALL
  SELECT delegated_groups.role
  FROM delegated_groups
  JOIN directory_group_memberships m ON m.group_id = delegated_groups.group_id
  JOIN directory_groups owning_group ON owning_group.id = m.group_id AND owning_group.company_id = $1::uuid
  JOIN domains owning_domain ON owning_domain.id = owning_group.domain_id AND owning_domain.company_id = owning_group.company_id
  JOIN companies owning_company ON owning_company.id = owning_group.company_id
  WHERE m.member_kind = $4
    AND m.member_id = $5::uuid
    AND ($7::boolean = false OR (
      m.status = 'active' AND owning_group.status = 'active' AND owning_domain.status = 'active' AND owning_company.status = 'active'
    ))
)
SELECT role FROM candidate_roles`
	rows, err := r.db.QueryContext(ctx, query,
		req.CompanyID,
		req.OwnerKind,
		req.OwnerID,
		req.DelegateKind,
		req.DelegateID,
		req.Scope,
		req.ActiveOnly,
		req.MaxDepth,
	)
	if err != nil {
		return false, fmt.Errorf("check effective directory delegation: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return false, fmt.Errorf("scan effective directory delegation: %w", err)
		}
		if DelegationRoleSatisfies(role, req.RequiredRole) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("check effective directory delegation rows: %w", err)
	}
	return false, nil
}

func (r *Repository) createDelegationTx(ctx context.Context, tx *sql.Tx, req CreateDelegationRequest) (Delegation, error) {
	const query = `
INSERT INTO directory_delegations (
  company_id,
  owner_kind,
  owner_id,
  delegate_kind,
  delegate_id,
  scope,
  role
)
VALUES ($1::uuid, $2, $3::uuid, $4, $5::uuid, $6, $7)
RETURNING id::text,
          company_id::text,
          owner_kind,
          owner_id::text,
          delegate_kind,
          delegate_id::text,
          scope,
          role,
          status`
	var delegation Delegation
	if err := tx.QueryRowContext(ctx, query,
		req.CompanyID,
		req.OwnerKind,
		req.OwnerID,
		req.DelegateKind,
		req.DelegateID,
		req.Scope,
		req.Role,
	).Scan(
		&delegation.ID,
		&delegation.CompanyID,
		&delegation.OwnerKind,
		&delegation.OwnerID,
		&delegation.DelegateKind,
		&delegation.DelegateID,
		&delegation.Scope,
		&delegation.Role,
		&delegation.Status,
	); err != nil {
		return Delegation{}, mapDirectoryDelegationInsertError(err)
	}
	return delegation, nil
}

func directoryDelegationCreateAuditDetail(delegation Delegation) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"delegation_id": delegation.ID,
		"company_id":    delegation.CompanyID,
		"owner_kind":    delegation.OwnerKind,
		"owner_id":      delegation.OwnerID,
		"delegate_kind": delegation.DelegateKind,
		"delegate_id":   delegation.DelegateID,
		"scope":         delegation.Scope,
		"role":          delegation.Role,
		"status":        delegation.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal directory delegation create audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) DeleteDelegationWithAudit(ctx context.Context, id string) (Delegation, error) {
	if r == nil || r.db == nil {
		return Delegation{}, fmt.Errorf("database handle is required")
	}
	id, err := NormalizePrincipalID(id)
	if err != nil {
		return Delegation{}, fmt.Errorf("delegation id: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Delegation{}, fmt.Errorf("begin delete directory delegation transaction: %w", err)
	}
	defer tx.Rollback()
	delegation, err := r.deleteDelegationTx(ctx, tx, id)
	if err != nil {
		return Delegation{}, err
	}
	detail, err := directoryDelegationDeleteAuditDetail(delegation)
	if err != nil {
		return Delegation{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  delegation.CompanyID,
		Category:   "admin",
		Action:     "directory_delegation.delete",
		TargetType: "directory_delegation",
		TargetID:   delegation.ID,
		Result:     "deleted",
		Detail:     detail,
	}); err != nil {
		return Delegation{}, fmt.Errorf("record directory delegation delete audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Delegation{}, fmt.Errorf("commit delete directory delegation transaction: %w", err)
	}
	return delegation, nil
}

func (r *Repository) deleteDelegationTx(ctx context.Context, tx *sql.Tx, id string) (Delegation, error) {
	const query = `
UPDATE directory_delegations
SET status = 'deleted',
    updated_at = now()
WHERE id = $1::uuid
  AND status = 'active'
RETURNING id::text,
          company_id::text,
          owner_kind,
          owner_id::text,
          delegate_kind,
          delegate_id::text,
          scope,
          role,
          status`
	var delegation Delegation
	if err := tx.QueryRowContext(ctx, query, id).Scan(
		&delegation.ID,
		&delegation.CompanyID,
		&delegation.OwnerKind,
		&delegation.OwnerID,
		&delegation.DelegateKind,
		&delegation.DelegateID,
		&delegation.Scope,
		&delegation.Role,
		&delegation.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Delegation{}, fmt.Errorf("directory delegation not found")
		}
		return Delegation{}, fmt.Errorf("delete directory delegation: %w", err)
	}
	return delegation, nil
}

func directoryDelegationDeleteAuditDetail(delegation Delegation) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"delegation_id":   delegation.ID,
		"company_id":      delegation.CompanyID,
		"owner_kind":      delegation.OwnerKind,
		"owner_id":        delegation.OwnerID,
		"delegate_kind":   delegation.DelegateKind,
		"delegate_id":     delegation.DelegateID,
		"scope":           delegation.Scope,
		"role":            delegation.Role,
		"previous_status": "active",
		"status":          delegation.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal directory delegation delete audit detail: %w", err)
	}
	return detail, nil
}

func mapDirectoryDelegationInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "idx_directory_delegations_active_grant":
			return fmt.Errorf("%w", ErrDelegationAlreadyExists)
		}
	}
	return fmt.Errorf("create directory delegation: %w", err)
}

func (r *Repository) ListDelegations(ctx context.Context, req ListDelegationsRequest) ([]Delegation, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeListDelegationsRequest(req)
	if err != nil {
		return nil, err
	}
	const query = `
SELECT id::text,
       company_id::text,
       owner_kind,
       owner_id::text,
       delegate_kind,
       delegate_id::text,
       scope,
       role,
       status
FROM directory_delegations
WHERE company_id = $1::uuid
  AND ($2 = '' OR owner_kind = $2)
  AND ($3 = '' OR owner_id = NULLIF($3, '')::uuid)
  AND ($4 = '' OR delegate_kind = $4)
  AND ($5 = '' OR delegate_id = NULLIF($5, '')::uuid)
  AND ($6 = '' OR scope = $6)
  AND ($7 = '' OR role = $7)
  AND ($8::boolean = false OR status = 'active')
ORDER BY updated_at DESC, id
LIMIT $9`
	rows, err := r.db.QueryContext(ctx, query,
		req.CompanyID,
		req.OwnerKind,
		req.OwnerID,
		req.DelegateKind,
		req.DelegateID,
		req.Scope,
		req.Role,
		req.ActiveOnly,
		req.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list directory delegations: %w", err)
	}
	defer rows.Close()
	delegations := make([]Delegation, 0, req.Limit)
	for rows.Next() {
		var delegation Delegation
		if err := rows.Scan(
			&delegation.ID,
			&delegation.CompanyID,
			&delegation.OwnerKind,
			&delegation.OwnerID,
			&delegation.DelegateKind,
			&delegation.DelegateID,
			&delegation.Scope,
			&delegation.Role,
			&delegation.Status,
		); err != nil {
			return nil, fmt.Errorf("scan directory delegation list result: %w", err)
		}
		delegations = append(delegations, delegation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list directory delegation rows: %w", err)
	}
	return delegations, nil
}

func (r *Repository) ensureDelegationPrincipalsActive(ctx context.Context, req CreateDelegationRequest) error {
	ok, err := r.checkDelegationPrincipalsActive(ctx, CheckDelegationRequest{
		CompanyID:    req.CompanyID,
		OwnerKind:    req.OwnerKind,
		OwnerID:      req.OwnerID,
		DelegateKind: req.DelegateKind,
		DelegateID:   req.DelegateID,
		Scope:        req.Scope,
		RequiredRole: req.Role,
		ActiveOnly:   true,
	})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("directory delegation principals must be active and belong to the same company")
	}
	return nil
}

func (r *Repository) checkDelegationPrincipalsActive(ctx context.Context, req CheckDelegationRequest) (bool, error) {
	owner, err := r.ResolvePrincipal(ctx, ResolvePrincipalRequest{
		ID:         req.OwnerID,
		Kind:       req.OwnerKind,
		ActiveOnly: true,
	})
	if err != nil {
		if errors.Is(err, ErrPrincipalNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("resolve active delegation owner: %w", err)
	}
	if owner.CompanyID != req.CompanyID {
		return false, nil
	}
	delegate, err := r.ResolvePrincipal(ctx, ResolvePrincipalRequest{
		ID:         req.DelegateID,
		Kind:       req.DelegateKind,
		ActiveOnly: true,
	})
	if err != nil {
		if errors.Is(err, ErrPrincipalNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("resolve active delegation delegate: %w", err)
	}
	if delegate.CompanyID != req.CompanyID {
		return false, nil
	}
	return true, nil
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
			return Principal{}, ErrPrincipalNotFound
		}
		return Principal{}, fmt.Errorf("resolve directory principal: %w", err)
	}
	return principal, nil
}

func searchPrincipalKindAllowed(kinds []string, kind string) bool {
	for _, candidate := range kinds {
		if candidate == kind {
			return true
		}
	}
	return false
}

func principalSearchPattern(query string) string {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(query) + 2)
	b.WriteByte('%')
	for _, r := range query {
		switch r {
		case '%', '_', '\\':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('%')
	return b.String()
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
			return Principal{}, ErrPrincipalNotFound
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
			return Principal{}, ErrPrincipalNotFound
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
			return Principal{}, ErrPrincipalNotFound
		}
		return Principal{}, fmt.Errorf("resolve directory principal: %w", err)
	}
	return principal, nil
}
