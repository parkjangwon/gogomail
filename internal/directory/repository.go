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

const resolveAliasBaseSQL = `
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
JOIN companies c ON c.id = a.company_id AND c.id = d.company_id`

func buildResolveAliasQuery(req ResolveAliasRequest) (string, []any) {
	conditions := []string{"lower(a.alias_address_ace) = $1"}
	if req.ActiveOnly {
		conditions = append(conditions, "(a.status = 'active' AND d.status = 'active' AND c.status = 'active')")
	}
	return resolveAliasBaseSQL + "\nWHERE " + strings.Join(conditions, "\n  AND "), []any{req.Address}
}

func (r *Repository) ResolveAlias(ctx context.Context, req ResolveAliasRequest) (Alias, error) {
	if r == nil || r.db == nil {
		return Alias{}, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeResolveAliasRequest(req)
	if err != nil {
		return Alias{}, err
	}
	query, args := buildResolveAliasQuery(req)
	var alias Alias
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
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
JOIN user_addresses a ON a.user_id = u.id AND lower(a.address) = lower($1)`

func buildResolveUserByEmailQuery(req ResolveUserByEmailRequest) (string, []any) {
	conditions := []string{}
	if req.ActiveOnly {
		conditions = append(conditions, "(u.status = 'active' AND d.status = 'active' AND c.status = 'active')")
	}
	query := resolveUserByEmailBaseSQL
	if len(conditions) > 0 {
		query += "\nWHERE " + strings.Join(conditions, "\n  AND ")
	}
	return query, []any{req.Email}
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

const listAliasesBaseSQL = `
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
JOIN companies c ON c.id = a.company_id AND c.id = d.company_id`

func buildListAliasesQuery(req ListAliasesRequest) (string, []any) {
	args := []any{req.CompanyID}
	conditions := []string{"a.company_id = $1::uuid"}
	if req.DomainID != "" {
		args = append(args, req.DomainID)
		conditions = append(conditions, fmt.Sprintf("a.domain_id = $%d::uuid", len(args)))
	}
	if req.TargetKind != "" {
		args = append(args, req.TargetKind)
		conditions = append(conditions, fmt.Sprintf("a.target_kind = $%d", len(args)))
	}
	if req.TargetID != "" {
		args = append(args, req.TargetID)
		conditions = append(conditions, fmt.Sprintf("a.target_id = $%d::uuid", len(args)))
	}
	if pattern := principalSearchPattern(req.Query); pattern != "" {
		args = append(args, pattern)
		conditions = append(conditions, fmt.Sprintf("(lower(a.alias_address) LIKE $%d ESCAPE '\\' OR lower(a.alias_address_ace) LIKE $%d ESCAPE '\\')", len(args), len(args)))
	}
	if req.ActiveOnly {
		conditions = append(conditions, "(a.status = 'active' AND d.status = 'active' AND c.status = 'active')")
	}
	args = append(args, req.Limit)
	query := listAliasesBaseSQL + "\nWHERE " + strings.Join(conditions, "\n  AND ") + fmt.Sprintf(`
ORDER BY lower(a.alias_address_ace), a.id
LIMIT $%d`, len(args))
	return query, args
}

func (r *Repository) ListAliases(ctx context.Context, req ListAliasesRequest) ([]Alias, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeListAliasesRequest(req)
	if err != nil {
		return nil, err
	}
	query, args := buildListAliasesQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
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

const listOrgTreeBaseSQL = `
SELECT o.id::text, o.name, COALESCE(o.parent_id::text, ''), o.depth, o.order_index
FROM organizations o
JOIN domains d ON d.id = o.domain_id
JOIN companies c ON c.id = d.company_id`

func buildListOrgTreeQuery(companyID, domainID string) (string, []any) {
	args := []any{strings.TrimSpace(companyID)}
	conditions := []string{"c.id = $1::uuid", "o.status = 'active'"}
	if domainID = strings.TrimSpace(domainID); domainID != "" {
		args = append(args, domainID)
		conditions = append(conditions, fmt.Sprintf("d.id = $%d::uuid", len(args)))
	}
	query := listOrgTreeBaseSQL + "\nWHERE " + strings.Join(conditions, "\n  AND ") + `
ORDER BY o.depth, o.order_index, lower(o.name)`
	return query, args
}

func (r *Repository) ListOrgTree(ctx context.Context, companyID, domainID string) ([]OrgTreeItem, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	query, args := buildListOrgTreeQuery(companyID, domainID)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list org tree: %w", err)
	}
	defer rows.Close()
	var result []OrgTreeItem
	for rows.Next() {
		var item OrgTreeItem
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.ParentID, &item.Depth, &item.OrderIndex); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) SearchPrincipals(ctx context.Context, req SearchPrincipalsRequest) ([]Principal, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeSearchPrincipalsRequest(req)
	if err != nil {
		return nil, err
	}
	query, args := buildSearchPrincipalsQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
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

func buildSearchPrincipalsQuery(req SearchPrincipalsRequest) (string, []any) {
	args := []any{req.CompanyID}
	var domainArg, orgArg, patternArg int
	if req.DomainID != "" {
		args = append(args, req.DomainID)
		domainArg = len(args)
	}
	if req.OrganizationID != "" {
		args = append(args, req.OrganizationID)
		orgArg = len(args)
	}
	if pattern := principalSearchPattern(req.Query); pattern != "" {
		args = append(args, pattern)
		patternArg = len(args)
	}

	branches := make([]string, 0, len(req.Kinds))
	if searchPrincipalKindAllowed(req.Kinds, PrincipalKindUser) {
		conditions := []string{"c.id = $1::uuid"}
		if domainArg != 0 {
			conditions = append(conditions, fmt.Sprintf("d.id = $%d::uuid", domainArg))
		}
		if orgArg != 0 {
			conditions = append(conditions, fmt.Sprintf("u.org_id = $%d::uuid", orgArg))
		}
		if req.ActiveOnly {
			conditions = append(conditions, "(u.status = 'active' AND d.status = 'active' AND c.status = 'active')")
		}
		if patternArg != 0 {
			conditions = append(conditions, fmt.Sprintf("(lower(u.display_name) LIKE $%d ESCAPE '\\' OR lower(u.username) LIKE $%d ESCAPE '\\' OR lower(COALESCE(primary_addr.address, '')) LIKE $%d ESCAPE '\\')", patternArg, patternArg, patternArg))
		}
		branches = append(branches, fmt.Sprintf(`
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
  WHERE %s`, strings.Join(conditions, "\n    AND ")))
	}
	if searchPrincipalKindAllowed(req.Kinds, PrincipalKindOrganization) {
		conditions := []string{"c.id = $1::uuid"}
		if domainArg != 0 {
			conditions = append(conditions, fmt.Sprintf("d.id = $%d::uuid", domainArg))
		}
		if orgArg != 0 {
			conditions = append(conditions, fmt.Sprintf("o.id = $%d::uuid", orgArg))
		}
		if req.ActiveOnly {
			conditions = append(conditions, "(o.status = 'active' AND d.status = 'active' AND c.status = 'active')")
		}
		if patternArg != 0 {
			conditions = append(conditions, fmt.Sprintf("(lower(o.name) LIKE $%d ESCAPE '\\' OR lower(o.code) LIKE $%d ESCAPE '\\')", patternArg, patternArg))
		}
		branches = append(branches, fmt.Sprintf(`
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
  WHERE %s`, strings.Join(conditions, "\n    AND ")))
	}
	if searchPrincipalKindAllowed(req.Kinds, PrincipalKindGroup) {
		conditions := []string{"g.company_id = $1::uuid"}
		if domainArg != 0 {
			conditions = append(conditions, fmt.Sprintf("g.domain_id = $%d::uuid", domainArg))
		}
		if orgArg != 0 {
			conditions = append(conditions, fmt.Sprintf("g.org_id = $%d::uuid", orgArg))
		}
		if req.ActiveOnly {
			conditions = append(conditions, "(g.status = 'active' AND d.status = 'active' AND c.status = 'active')")
		}
		if patternArg != 0 {
			conditions = append(conditions, fmt.Sprintf("(lower(g.name) LIKE $%d ESCAPE '\\' OR lower(g.slug) LIKE $%d ESCAPE '\\')", patternArg, patternArg))
		}
		branches = append(branches, fmt.Sprintf(`
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
  WHERE %s`, strings.Join(conditions, "\n    AND ")))
	}
	if searchPrincipalKindAllowed(req.Kinds, PrincipalKindResource) {
		conditions := []string{"rsrc.company_id = $1::uuid"}
		if domainArg != 0 {
			conditions = append(conditions, fmt.Sprintf("rsrc.domain_id = $%d::uuid", domainArg))
		}
		if orgArg != 0 {
			conditions = append(conditions, fmt.Sprintf("rsrc.org_id = $%d::uuid", orgArg))
		}
		if req.ActiveOnly {
			conditions = append(conditions, "(rsrc.status = 'active' AND d.status = 'active' AND c.status = 'active')")
		}
		if patternArg != 0 {
			conditions = append(conditions, fmt.Sprintf("(lower(rsrc.name) LIKE $%d ESCAPE '\\' OR lower(rsrc.slug) LIKE $%d ESCAPE '\\' OR lower(rsrc.resource_type) LIKE $%d ESCAPE '\\')", patternArg, patternArg, patternArg))
		}
		branches = append(branches, fmt.Sprintf(`
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
  WHERE %s`, strings.Join(conditions, "\n    AND ")))
	}

	args = append(args, req.Limit, req.Offset)
	limitArg := len(args) - 1
	offsetArg := len(args)
	query := `
WITH principals AS (` + strings.Join(branches, "\n  UNION ALL") + `
)
SELECT id, kind, company_id, domain_id, organization_id, display_name, primary_email, status, resource_type
FROM principals
ORDER BY kind_rank, lower(display_name), id
LIMIT $%d OFFSET $%d`
	return fmt.Sprintf(query, limitArg, offsetArg), args
}

func buildCheckDirectGroupMembershipQuery(req CheckGroupMembershipRequest) (string, []any) {
	conditions := []string{
		"m.group_id = $1::uuid",
		"m.member_kind = $2",
		"m.member_id = $3::uuid",
	}
	if req.ActiveOnly {
		conditions = append(conditions, "(m.status = 'active' AND g.status = 'active' AND d.status = 'active' AND c.status = 'active')")
	}
	query := `
SELECT EXISTS (
  SELECT 1
  FROM directory_group_memberships m
  JOIN directory_groups g ON g.id = m.group_id
  JOIN domains d ON d.id = g.domain_id
  JOIN companies c ON c.id = g.company_id AND c.id = d.company_id
  WHERE ` + strings.Join(conditions, "\n    AND ") + `
)`
	return query, []any{req.GroupID, req.MemberKind, req.MemberID}
}

func (r *Repository) CheckDirectGroupMembership(ctx context.Context, req CheckGroupMembershipRequest) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeCheckGroupMembershipRequest(req)
	if err != nil {
		return false, err
	}
	query, args := buildCheckDirectGroupMembershipQuery(req)
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&exists); err != nil {
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

func buildCheckEffectiveGroupMembershipQuery(req CheckGroupMembershipRequest) (string, []any) {
	nestedConditions := []string{
		"m.member_kind = 'group'",
		"group_tree.depth < $4",
		"NOT m.member_id = ANY(group_tree.path)",
	}
	memberConditions := []string{
		"m.member_kind = $2",
		"m.member_id = $3::uuid",
	}
	if req.ActiveOnly {
		nestedConditions = append(nestedConditions, "(m.status = 'active' AND nested_group.status = 'active' AND d.status = 'active' AND c.status = 'active')")
		memberConditions = append(memberConditions, "(m.status = 'active' AND owning_group.status = 'active' AND d.status = 'active' AND c.status = 'active')")
	}
	query := `
WITH RECURSIVE group_tree(group_id, depth, path) AS (
  SELECT $1::uuid, 0, ARRAY[$1::uuid]
  UNION ALL
  SELECT m.member_id, group_tree.depth + 1, group_tree.path || m.member_id
  FROM group_tree
  JOIN directory_group_memberships m ON m.group_id = group_tree.group_id
  JOIN directory_groups nested_group ON nested_group.id = m.member_id
  JOIN domains d ON d.id = nested_group.domain_id
  JOIN companies c ON c.id = nested_group.company_id AND c.id = d.company_id
  WHERE ` + strings.Join(nestedConditions, "\n    AND ") + `
)
SELECT EXISTS (
  SELECT 1
  FROM group_tree
  JOIN directory_group_memberships m ON m.group_id = group_tree.group_id
  JOIN directory_groups owning_group ON owning_group.id = m.group_id
  JOIN domains d ON d.id = owning_group.domain_id
  JOIN companies c ON c.id = owning_group.company_id AND c.id = d.company_id
  WHERE ` + strings.Join(memberConditions, "\n    AND ") + `
)`
	return query, []any{req.GroupID, req.MemberKind, req.MemberID, req.MaxDepth}
}

func (r *Repository) CheckEffectiveGroupMembership(ctx context.Context, req CheckGroupMembershipRequest) (bool, error) {
	if r == nil || r.db == nil {
		return false, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeCheckGroupMembershipRequest(req)
	if err != nil {
		return false, err
	}
	query, args := buildCheckEffectiveGroupMembershipQuery(req)
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&exists); err != nil {
		return false, fmt.Errorf("check effective group membership: %w", err)
	}
	return exists, nil
}

const listGroupMembershipsBaseSQL = `
SELECT m.id::text,
       m.group_id::text,
       g.company_id::text,
       m.member_kind,
       m.member_id::text,
       m.role,
       m.status
FROM directory_group_memberships m
JOIN directory_groups g ON g.id = m.group_id
JOIN domains d ON d.id = g.domain_id AND d.company_id = g.company_id
JOIN companies c ON c.id = g.company_id`

func buildListGroupMembershipsQuery(req ListGroupMembershipsRequest) (string, []any) {
	args := []any{req.CompanyID}
	conditions := []string{"g.company_id = $1::uuid"}
	if req.GroupID != "" {
		args = append(args, req.GroupID)
		conditions = append(conditions, fmt.Sprintf("m.group_id = $%d::uuid", len(args)))
	}
	if req.MemberKind != "" {
		args = append(args, req.MemberKind)
		conditions = append(conditions, fmt.Sprintf("m.member_kind = $%d", len(args)))
	}
	if req.MemberID != "" {
		args = append(args, req.MemberID)
		conditions = append(conditions, fmt.Sprintf("m.member_id = $%d::uuid", len(args)))
	}
	if req.Role != "" {
		args = append(args, req.Role)
		conditions = append(conditions, fmt.Sprintf("m.role = $%d", len(args)))
	}
	if req.ActiveOnly {
		conditions = append(conditions, "(m.status = 'active' AND g.status = 'active' AND d.status = 'active' AND c.status = 'active')")
	}
	args = append(args, req.Limit)
	query := listGroupMembershipsBaseSQL + "\nWHERE " + strings.Join(conditions, "\n  AND ") + fmt.Sprintf(`
ORDER BY m.updated_at DESC, m.id
LIMIT $%d`, len(args))
	return query, args
}

func (r *Repository) ListGroupMemberships(ctx context.Context, req ListGroupMembershipsRequest) ([]GroupMembership, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeListGroupMembershipsRequest(req)
	if err != nil {
		return nil, err
	}
	query, args := buildListGroupMembershipsQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list directory group memberships: %w", err)
	}
	defer rows.Close()
	memberships := make([]GroupMembership, 0, req.Limit)
	for rows.Next() {
		var membership GroupMembership
		if err := rows.Scan(
			&membership.ID,
			&membership.GroupID,
			&membership.CompanyID,
			&membership.MemberKind,
			&membership.MemberID,
			&membership.Role,
			&membership.Status,
		); err != nil {
			return nil, fmt.Errorf("scan directory group membership list result: %w", err)
		}
		memberships = append(memberships, membership)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list directory group membership rows: %w", err)
	}
	return memberships, nil
}

const listGroupMembershipsForGroupsSQL = `
SELECT req.group_order,
       m.updated_at,
       m.id::text,
       m.group_id::text,
       g.company_id::text,
       m.member_kind,
       m.member_id::text,
       m.role,
       m.status
FROM directory_group_memberships m
JOIN directory_groups g ON g.id = m.group_id
JOIN domains d ON d.id = g.domain_id AND d.company_id = g.company_id
JOIN companies c ON c.id = g.company_id
JOIN unnest($2::uuid[]) WITH ORDINALITY AS req(group_id, group_order) ON req.group_id = m.group_id
WHERE g.company_id = $1::uuid`

func buildListGroupMembershipsForGroupsQuery(activeOnly bool) string {
	query := `
WITH ranked_memberships AS (
` + listGroupMembershipsForGroupsSQL
	if activeOnly {
		query += `
  AND (m.status = 'active' AND g.status = 'active' AND d.status = 'active' AND c.status = 'active')`
	}
	query += `
)
SELECT id, group_id, company_id, member_kind, member_id, role, status
FROM (
  SELECT ranked_memberships.*,
         row_number() OVER (PARTITION BY group_id ORDER BY updated_at DESC, id) AS rn
  FROM ranked_memberships
) limited
WHERE rn <= $3
ORDER BY group_order, rn`
	return query
}

func (r *Repository) ListGroupMembershipsForGroups(ctx context.Context, companyID string, groupIDs []string, activeOnly bool, perGroupLimit int) (map[string][]GroupMembership, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	companyID, err := NormalizePrincipalID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company id: %w", err)
	}
	groupIDs, err = normalizePrincipalIDList(groupIDs)
	if err != nil {
		return nil, fmt.Errorf("group ids: %w", err)
	}
	perGroupLimit, err = normalizeMembershipBatchLimit(perGroupLimit)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]GroupMembership, len(groupIDs))
	if len(groupIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, buildListGroupMembershipsForGroupsQuery(activeOnly), companyID, pq.Array(groupIDs), perGroupLimit)
	if err != nil {
		return nil, fmt.Errorf("list directory group memberships for groups: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		membership, err := scanGroupMembership(rows)
		if err != nil {
			return nil, err
		}
		out[membership.GroupID] = append(out[membership.GroupID], membership)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list directory group memberships for groups rows: %w", err)
	}
	return out, nil
}

const listGroupMembershipsForMembersSQL = `
SELECT req.member_order,
       m.updated_at,
       m.id::text,
       m.group_id::text,
       g.company_id::text,
       m.member_kind,
       m.member_id::text,
       m.role,
       m.status
FROM directory_group_memberships m
JOIN directory_groups g ON g.id = m.group_id
JOIN domains d ON d.id = g.domain_id AND d.company_id = g.company_id
JOIN companies c ON c.id = g.company_id
JOIN unnest($2::text[], $3::uuid[]) WITH ORDINALITY AS req(member_kind, member_id, member_order)
  ON req.member_kind = m.member_kind AND req.member_id = m.member_id
WHERE g.company_id = $1::uuid`

func buildListGroupMembershipsForMembersQuery(activeOnly bool) string {
	query := `
WITH ranked_memberships AS (
` + listGroupMembershipsForMembersSQL
	if activeOnly {
		query += `
  AND (m.status = 'active' AND g.status = 'active' AND d.status = 'active' AND c.status = 'active')`
	}
	query += `
)
SELECT id, group_id, company_id, member_kind, member_id, role, status
FROM (
  SELECT ranked_memberships.*,
         row_number() OVER (PARTITION BY member_kind, member_id ORDER BY updated_at DESC, id) AS rn
  FROM ranked_memberships
) limited
WHERE rn <= $4
ORDER BY member_order, rn`
	return query
}

func (r *Repository) ListGroupMembershipsForMembers(ctx context.Context, companyID string, members []PrincipalRef, activeOnly bool, perMemberLimit int) (map[PrincipalRef][]GroupMembership, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	companyID, err := NormalizePrincipalID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company id: %w", err)
	}
	members, err = normalizePrincipalRefs(members)
	if err != nil {
		return nil, err
	}
	perMemberLimit, err = normalizeMembershipBatchLimit(perMemberLimit)
	if err != nil {
		return nil, err
	}
	out := make(map[PrincipalRef][]GroupMembership, len(members))
	if len(members) == 0 {
		return out, nil
	}
	kinds := make([]string, 0, len(members))
	ids := make([]string, 0, len(members))
	for _, member := range members {
		kinds = append(kinds, member.Kind)
		ids = append(ids, member.ID)
	}
	rows, err := r.db.QueryContext(ctx, buildListGroupMembershipsForMembersQuery(activeOnly), companyID, pq.Array(kinds), pq.Array(ids), perMemberLimit)
	if err != nil {
		return nil, fmt.Errorf("list directory group memberships for members: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		membership, err := scanGroupMembership(rows)
		if err != nil {
			return nil, err
		}
		key := PrincipalRef{Kind: membership.MemberKind, ID: membership.MemberID}
		out[key] = append(out[key], membership)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list directory group memberships for members rows: %w", err)
	}
	return out, nil
}

type groupMembershipScanner interface {
	Scan(dest ...any) error
}

func scanGroupMembership(row groupMembershipScanner) (GroupMembership, error) {
	var membership GroupMembership
	if err := row.Scan(
		&membership.ID,
		&membership.GroupID,
		&membership.CompanyID,
		&membership.MemberKind,
		&membership.MemberID,
		&membership.Role,
		&membership.Status,
	); err != nil {
		return GroupMembership{}, fmt.Errorf("scan directory group membership list result: %w", err)
	}
	return membership, nil
}

func normalizePrincipalIDList(ids []string) ([]string, error) {
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		normalized, err := NormalizePrincipalID(id)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out, nil
}

func normalizePrincipalRefs(refs []PrincipalRef) ([]PrincipalRef, error) {
	out := make([]PrincipalRef, 0, len(refs))
	seen := make(map[PrincipalRef]struct{}, len(refs))
	for _, ref := range refs {
		kind, err := NormalizePrincipalKind(ref.Kind)
		if err != nil {
			return nil, fmt.Errorf("member kind: %w", err)
		}
		id, err := NormalizePrincipalID(ref.ID)
		if err != nil {
			return nil, fmt.Errorf("member id: %w", err)
		}
		normalized := PrincipalRef{Kind: kind, ID: id}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeMembershipBatchLimit(limit int) (int, error) {
	if limit < 0 {
		return 0, fmt.Errorf("group membership list limit must not be negative")
	}
	if limit == 0 {
		return DefaultGroupMembershipListLimit, nil
	}
	if limit > MaxGroupMembershipListLimit {
		return 0, fmt.Errorf("group membership list limit is too large")
	}
	return limit, nil
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

func (r *Repository) UpdateGroupMembershipRoleWithAudit(ctx context.Context, req UpdateGroupMembershipRoleRequest) (GroupMembership, error) {
	if r == nil || r.db == nil {
		return GroupMembership{}, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeUpdateGroupMembershipRoleRequest(req)
	if err != nil {
		return GroupMembership{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return GroupMembership{}, fmt.Errorf("begin update directory group membership role transaction: %w", err)
	}
	defer tx.Rollback()
	membership, previousRole, err := r.updateGroupMembershipRoleTx(ctx, tx, req)
	if err != nil {
		return GroupMembership{}, err
	}
	detail, err := directoryGroupMembershipRoleUpdateAuditDetail(membership, previousRole)
	if err != nil {
		return GroupMembership{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  membership.CompanyID,
		Category:   "admin",
		Action:     "directory_group_membership.role_update",
		TargetType: "directory_group_membership",
		TargetID:   membership.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return GroupMembership{}, fmt.Errorf("record directory group membership role update audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return GroupMembership{}, fmt.Errorf("commit update directory group membership role transaction: %w", err)
	}
	return membership, nil
}

func (r *Repository) updateGroupMembershipRoleTx(ctx context.Context, tx *sql.Tx, req UpdateGroupMembershipRoleRequest) (GroupMembership, string, error) {
	const query = `
WITH current_membership AS (
  SELECT m.id,
         m.role AS previous_role
  FROM directory_group_memberships m
  JOIN directory_groups g ON g.id = m.group_id
  JOIN domains d ON d.id = g.domain_id AND d.company_id = g.company_id
  JOIN companies c ON c.id = g.company_id
  WHERE m.id = $1::uuid
    AND m.status = 'active'
    AND g.status = 'active'
    AND d.status = 'active'
    AND c.status = 'active'
  FOR UPDATE OF m
)
UPDATE directory_group_memberships AS m
SET role = $2,
    updated_at = now()
FROM current_membership AS current,
     directory_groups AS g
WHERE m.id = current.id
  AND g.id = m.group_id
RETURNING m.id::text,
          m.group_id::text,
          g.company_id::text,
          m.member_kind,
          m.member_id::text,
          m.role,
          m.status,
          current.previous_role`
	var membership GroupMembership
	var previousRole string
	if err := tx.QueryRowContext(ctx, query, req.ID, req.Role).Scan(
		&membership.ID,
		&membership.GroupID,
		&membership.CompanyID,
		&membership.MemberKind,
		&membership.MemberID,
		&membership.Role,
		&membership.Status,
		&previousRole,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GroupMembership{}, "", fmt.Errorf("directory group membership not found")
		}
		return GroupMembership{}, "", fmt.Errorf("update directory group membership role: %w", err)
	}
	return membership, previousRole, nil
}

func directoryGroupMembershipRoleUpdateAuditDetail(membership GroupMembership, previousRole string) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"membership_id": membership.ID,
		"group_id":      membership.GroupID,
		"company_id":    membership.CompanyID,
		"member_kind":   membership.MemberKind,
		"member_id":     membership.MemberID,
		"previous_role": previousRole,
		"role":          membership.Role,
		"status":        membership.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal directory group membership role update audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ReassignGroupMembershipWithAudit(ctx context.Context, req ReassignGroupMembershipRequest) (GroupMembership, error) {
	if r == nil || r.db == nil {
		return GroupMembership{}, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeReassignGroupMembershipRequest(req)
	if err != nil {
		return GroupMembership{}, err
	}
	companyID, err := r.ensureGroupMembershipReassignPrincipalsActive(ctx, req)
	if err != nil {
		return GroupMembership{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return GroupMembership{}, fmt.Errorf("begin reassign directory group membership transaction: %w", err)
	}
	defer tx.Rollback()
	membership, previous, err := r.reassignGroupMembershipTx(ctx, tx, req, companyID)
	if err != nil {
		return GroupMembership{}, err
	}
	detail, err := directoryGroupMembershipReassignAuditDetail(membership, previous)
	if err != nil {
		return GroupMembership{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  membership.CompanyID,
		Category:   "admin",
		Action:     "directory_group_membership.reassign",
		TargetType: "directory_group_membership",
		TargetID:   membership.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return GroupMembership{}, fmt.Errorf("record directory group membership reassign audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return GroupMembership{}, fmt.Errorf("commit reassign directory group membership transaction: %w", err)
	}
	return membership, nil
}

func (r *Repository) reassignGroupMembershipTx(ctx context.Context, tx *sql.Tx, req ReassignGroupMembershipRequest, companyID string) (GroupMembership, GroupMembership, error) {
	const query = `
WITH current_membership AS (
  SELECT m.id,
         m.group_id,
         m.member_kind,
         m.member_id,
         m.role
  FROM directory_group_memberships m
  JOIN directory_groups current_group ON current_group.id = m.group_id
  WHERE m.id = $1::uuid
    AND m.status = 'active'
    AND current_group.company_id = $5::uuid
  FOR UPDATE
)
UPDATE directory_group_memberships AS m
SET group_id = $2::uuid,
    member_kind = $3,
    member_id = $4::uuid,
    updated_at = now()
FROM current_membership AS current
WHERE m.id = current.id
RETURNING m.id::text,
          m.group_id::text,
          m.member_kind,
          m.member_id::text,
          m.role,
          m.status,
          current.group_id::text,
          current.member_kind,
          current.member_id::text,
          current.role`
	var membership GroupMembership
	var previous GroupMembership
	if err := tx.QueryRowContext(ctx, query,
		req.ID,
		req.GroupID,
		req.MemberKind,
		req.MemberID,
		companyID,
	).Scan(
		&membership.ID,
		&membership.GroupID,
		&membership.MemberKind,
		&membership.MemberID,
		&membership.Role,
		&membership.Status,
		&previous.GroupID,
		&previous.MemberKind,
		&previous.MemberID,
		&previous.Role,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GroupMembership{}, GroupMembership{}, fmt.Errorf("directory group membership not found")
		}
		return GroupMembership{}, GroupMembership{}, mapDirectoryGroupMembershipUpdateError(err)
	}
	membership.CompanyID = companyID
	previous.ID = membership.ID
	previous.CompanyID = companyID
	previous.Status = "active"
	return membership, previous, nil
}

func directoryGroupMembershipReassignAuditDetail(membership GroupMembership, previous GroupMembership) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"membership_id":        membership.ID,
		"company_id":           membership.CompanyID,
		"previous_group_id":    previous.GroupID,
		"group_id":             membership.GroupID,
		"previous_member_kind": previous.MemberKind,
		"member_kind":          membership.MemberKind,
		"previous_member_id":   previous.MemberID,
		"member_id":            membership.MemberID,
		"role":                 membership.Role,
		"status":               membership.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal directory group membership reassign audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) DeleteGroupMembershipWithAudit(ctx context.Context, id string) (GroupMembership, error) {
	if r == nil || r.db == nil {
		return GroupMembership{}, fmt.Errorf("database handle is required")
	}
	id, err := NormalizePrincipalID(id)
	if err != nil {
		return GroupMembership{}, fmt.Errorf("membership id: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return GroupMembership{}, fmt.Errorf("begin delete directory group membership transaction: %w", err)
	}
	defer tx.Rollback()
	membership, err := r.deleteGroupMembershipTx(ctx, tx, id)
	if err != nil {
		return GroupMembership{}, err
	}
	detail, err := directoryGroupMembershipDeleteAuditDetail(membership)
	if err != nil {
		return GroupMembership{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  membership.CompanyID,
		Category:   "admin",
		Action:     "directory_group_membership.delete",
		TargetType: "directory_group_membership",
		TargetID:   membership.ID,
		Result:     "deleted",
		Detail:     detail,
	}); err != nil {
		return GroupMembership{}, fmt.Errorf("record directory group membership delete audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return GroupMembership{}, fmt.Errorf("commit delete directory group membership transaction: %w", err)
	}
	return membership, nil
}

func (r *Repository) deleteGroupMembershipTx(ctx context.Context, tx *sql.Tx, id string) (GroupMembership, error) {
	const query = `
UPDATE directory_group_memberships AS m
SET status = 'deleted',
    updated_at = now()
FROM directory_groups AS g
WHERE m.id = $1::uuid
  AND m.status = 'active'
  AND g.id = m.group_id
RETURNING m.id::text,
          m.group_id::text,
          g.company_id::text,
          m.member_kind,
          m.member_id::text,
          m.role,
          m.status`
	var membership GroupMembership
	if err := tx.QueryRowContext(ctx, query, id).Scan(
		&membership.ID,
		&membership.GroupID,
		&membership.CompanyID,
		&membership.MemberKind,
		&membership.MemberID,
		&membership.Role,
		&membership.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GroupMembership{}, fmt.Errorf("directory group membership not found")
		}
		return GroupMembership{}, fmt.Errorf("delete directory group membership: %w", err)
	}
	return membership, nil
}

func directoryGroupMembershipDeleteAuditDetail(membership GroupMembership) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"membership_id":   membership.ID,
		"group_id":        membership.GroupID,
		"company_id":      membership.CompanyID,
		"member_kind":     membership.MemberKind,
		"member_id":       membership.MemberID,
		"role":            membership.Role,
		"previous_status": "active",
		"status":          membership.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal directory group membership delete audit detail: %w", err)
	}
	return detail, nil
}

func mapDirectoryGroupMembershipInsertError(err error) error {
	return mapDirectoryGroupMembershipWriteError("create directory group membership", err)
}

func mapDirectoryGroupMembershipUpdateError(err error) error {
	return mapDirectoryGroupMembershipWriteError("update directory group membership", err)
}

func mapDirectoryGroupMembershipWriteError(operation string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "idx_directory_group_memberships_active_member":
			return fmt.Errorf("%w", ErrGroupMembershipAlreadyExists)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
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

func (r *Repository) ensureGroupMembershipReassignPrincipalsActive(ctx context.Context, req ReassignGroupMembershipRequest) (string, error) {
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
		wouldCycle, err := r.checkEffectiveGroupMembershipExcluding(ctx, CheckGroupMembershipRequest{
			GroupID:    req.MemberID,
			MemberKind: PrincipalKindGroup,
			MemberID:   req.GroupID,
			ActiveOnly: true,
			MaxDepth:   MaxGroupMembershipDepth,
		}, req.ID)
		if err != nil {
			return "", fmt.Errorf("check directory group membership cycle: %w", err)
		}
		if wouldCycle {
			return "", fmt.Errorf("directory group membership would create a cycle")
		}
	}
	return group.CompanyID, nil
}

func (r *Repository) checkEffectiveGroupMembershipExcluding(ctx context.Context, req CheckGroupMembershipRequest, excludeMembershipID string) (bool, error) {
	req, err := NormalizeCheckGroupMembershipRequest(req)
	if err != nil {
		return false, err
	}
	excludeMembershipID, err = NormalizePrincipalID(excludeMembershipID)
	if err != nil {
		return false, fmt.Errorf("excluded membership id: %w", err)
	}
	query, args := buildCheckEffectiveGroupMembershipExcludingQuery(req, excludeMembershipID)
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&exists); err != nil {
		return false, fmt.Errorf("check effective group membership excluding current membership: %w", err)
	}
	return exists, nil
}

func buildCheckEffectiveGroupMembershipExcludingQuery(req CheckGroupMembershipRequest, excludeMembershipID string) (string, []any) {
	nestedConditions := []string{
		"m.id <> $5::uuid",
		"m.member_kind = 'group'",
		"group_tree.depth < $4",
		"NOT m.member_id = ANY(group_tree.path)",
	}
	memberConditions := []string{
		"m.id <> $5::uuid",
		"m.member_kind = $2",
		"m.member_id = $3::uuid",
	}
	if req.ActiveOnly {
		nestedConditions = append(nestedConditions, "(m.status = 'active' AND nested_group.status = 'active' AND d.status = 'active' AND c.status = 'active')")
		memberConditions = append(memberConditions, "(m.status = 'active' AND owning_group.status = 'active' AND d.status = 'active' AND c.status = 'active')")
	}
	query := `
WITH RECURSIVE group_tree(group_id, depth, path) AS (
  SELECT $1::uuid, 0, ARRAY[$1::uuid]
  UNION ALL
  SELECT m.member_id, group_tree.depth + 1, group_tree.path || m.member_id
  FROM group_tree
  JOIN directory_group_memberships m ON m.group_id = group_tree.group_id
  JOIN directory_groups nested_group ON nested_group.id = m.member_id
  JOIN domains d ON d.id = nested_group.domain_id
  JOIN companies c ON c.id = nested_group.company_id AND c.id = d.company_id
  WHERE ` + strings.Join(nestedConditions, "\n    AND ") + `
)
SELECT EXISTS (
  SELECT 1
  FROM group_tree
  JOIN directory_group_memberships m ON m.group_id = group_tree.group_id
  JOIN directory_groups owning_group ON owning_group.id = m.group_id
  JOIN domains d ON d.id = owning_group.domain_id
  JOIN companies c ON c.id = owning_group.company_id AND c.id = d.company_id
  WHERE ` + strings.Join(memberConditions, "\n    AND ") + `
)`
	return query, []any{req.GroupID, req.MemberKind, req.MemberID, req.MaxDepth, excludeMembershipID}
}

func buildCheckDelegationQuery(req CheckDelegationRequest) (string, []any) {
	conditions := []string{
		"d.company_id = $1::uuid",
		"d.owner_kind = $2",
		"d.owner_id = $3::uuid",
		"d.delegate_kind = $4",
		"d.delegate_id = $5::uuid",
		"d.scope = $6",
	}
	if req.ActiveOnly {
		conditions = append(conditions, "(d.status = 'active' AND c.status = 'active')")
	}
	query := `
SELECT d.role
FROM directory_delegations d
JOIN companies c ON c.id = d.company_id
WHERE ` + strings.Join(conditions, "\n  AND ")
	return query, []any{
		req.CompanyID,
		req.OwnerKind,
		req.OwnerID,
		req.DelegateKind,
		req.DelegateID,
		req.Scope,
	}
}

func (r *Repository) CheckDelegation(ctx context.Context, req CheckDelegationRequest) (bool, error) {
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
	query, args := buildCheckDelegationQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
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

func (r *Repository) UpdateDelegationRoleWithAudit(ctx context.Context, req UpdateDelegationRoleRequest) (Delegation, error) {
	if r == nil || r.db == nil {
		return Delegation{}, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeUpdateDelegationRoleRequest(req)
	if err != nil {
		return Delegation{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Delegation{}, fmt.Errorf("begin update directory delegation role transaction: %w", err)
	}
	defer tx.Rollback()
	delegation, previousRole, err := r.updateDelegationRoleTx(ctx, tx, req)
	if err != nil {
		return Delegation{}, err
	}
	detail, err := directoryDelegationRoleUpdateAuditDetail(delegation, previousRole)
	if err != nil {
		return Delegation{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  delegation.CompanyID,
		Category:   "admin",
		Action:     "directory_delegation.role_update",
		TargetType: "directory_delegation",
		TargetID:   delegation.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return Delegation{}, fmt.Errorf("record directory delegation role update audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Delegation{}, fmt.Errorf("commit update directory delegation role transaction: %w", err)
	}
	return delegation, nil
}

func (r *Repository) updateDelegationRoleTx(ctx context.Context, tx *sql.Tx, req UpdateDelegationRoleRequest) (Delegation, string, error) {
	const query = `
WITH current_delegation AS (
  SELECT d.id,
         d.role AS previous_role
  FROM directory_delegations d
  JOIN companies c ON c.id = d.company_id
  WHERE d.id = $1::uuid
    AND d.status = 'active'
    AND c.status = 'active'
  FOR UPDATE OF d
)
UPDATE directory_delegations AS d
SET role = $2,
    updated_at = now()
FROM current_delegation AS current
WHERE d.id = current.id
RETURNING d.id::text,
          d.company_id::text,
          d.owner_kind,
          d.owner_id::text,
          d.delegate_kind,
          d.delegate_id::text,
          d.scope,
          d.role,
          d.status,
          current.previous_role`
	var delegation Delegation
	var previousRole string
	if err := tx.QueryRowContext(ctx, query, req.ID, req.Role).Scan(
		&delegation.ID,
		&delegation.CompanyID,
		&delegation.OwnerKind,
		&delegation.OwnerID,
		&delegation.DelegateKind,
		&delegation.DelegateID,
		&delegation.Scope,
		&delegation.Role,
		&delegation.Status,
		&previousRole,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Delegation{}, "", fmt.Errorf("directory delegation not found")
		}
		return Delegation{}, "", mapDirectoryDelegationUpdateError(err)
	}
	return delegation, previousRole, nil
}

func directoryDelegationRoleUpdateAuditDetail(delegation Delegation, previousRole string) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"delegation_id": delegation.ID,
		"company_id":    delegation.CompanyID,
		"owner_kind":    delegation.OwnerKind,
		"owner_id":      delegation.OwnerID,
		"delegate_kind": delegation.DelegateKind,
		"delegate_id":   delegation.DelegateID,
		"scope":         delegation.Scope,
		"previous_role": previousRole,
		"role":          delegation.Role,
		"status":        delegation.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal directory delegation role update audit detail: %w", err)
	}
	return detail, nil
}

func (r *Repository) ReassignDelegationWithAudit(ctx context.Context, req ReassignDelegationRequest) (Delegation, error) {
	if r == nil || r.db == nil {
		return Delegation{}, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeReassignDelegationRequest(req)
	if err != nil {
		return Delegation{}, err
	}
	companyID, err := r.ensureDelegationReassignPrincipalsActive(ctx, req)
	if err != nil {
		return Delegation{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Delegation{}, fmt.Errorf("begin reassign directory delegation transaction: %w", err)
	}
	defer tx.Rollback()
	delegation, previous, err := r.reassignDelegationTx(ctx, tx, req, companyID)
	if err != nil {
		return Delegation{}, err
	}
	detail, err := directoryDelegationReassignAuditDetail(delegation, previous)
	if err != nil {
		return Delegation{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  delegation.CompanyID,
		Category:   "admin",
		Action:     "directory_delegation.reassign",
		TargetType: "directory_delegation",
		TargetID:   delegation.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return Delegation{}, fmt.Errorf("record directory delegation reassign audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Delegation{}, fmt.Errorf("commit reassign directory delegation transaction: %w", err)
	}
	return delegation, nil
}

func (r *Repository) reassignDelegationTx(ctx context.Context, tx *sql.Tx, req ReassignDelegationRequest, companyID string) (Delegation, Delegation, error) {
	const query = `
WITH current_delegation AS (
  SELECT d.id,
         d.company_id,
         d.owner_kind,
         d.owner_id,
         d.delegate_kind,
         d.delegate_id,
         d.scope,
         d.role
  FROM directory_delegations d
  JOIN companies c ON c.id = d.company_id
  WHERE d.id = $1::uuid
    AND d.status = 'active'
    AND d.company_id = $7::uuid
    AND c.status = 'active'
  FOR UPDATE OF d
)
UPDATE directory_delegations AS d
SET owner_kind = $2,
    owner_id = $3::uuid,
    delegate_kind = $4,
    delegate_id = $5::uuid,
    scope = $6,
    updated_at = now()
FROM current_delegation AS current
WHERE d.id = current.id
RETURNING d.id::text,
          d.company_id::text,
          d.owner_kind,
          d.owner_id::text,
          d.delegate_kind,
          d.delegate_id::text,
          d.scope,
          d.role,
          d.status,
          current.owner_kind,
          current.owner_id::text,
          current.delegate_kind,
          current.delegate_id::text,
          current.scope,
          current.role`
	var delegation Delegation
	var previous Delegation
	if err := tx.QueryRowContext(ctx, query,
		req.ID,
		req.OwnerKind,
		req.OwnerID,
		req.DelegateKind,
		req.DelegateID,
		req.Scope,
		companyID,
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
		&previous.OwnerKind,
		&previous.OwnerID,
		&previous.DelegateKind,
		&previous.DelegateID,
		&previous.Scope,
		&previous.Role,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Delegation{}, Delegation{}, fmt.Errorf("directory delegation not found")
		}
		return Delegation{}, Delegation{}, mapDirectoryDelegationUpdateError(err)
	}
	previous.ID = delegation.ID
	previous.CompanyID = delegation.CompanyID
	previous.Status = "active"
	return delegation, previous, nil
}

func directoryDelegationReassignAuditDetail(delegation Delegation, previous Delegation) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"delegation_id":          delegation.ID,
		"company_id":             delegation.CompanyID,
		"previous_owner_kind":    previous.OwnerKind,
		"owner_kind":             delegation.OwnerKind,
		"previous_owner_id":      previous.OwnerID,
		"owner_id":               delegation.OwnerID,
		"previous_delegate_kind": previous.DelegateKind,
		"delegate_kind":          delegation.DelegateKind,
		"previous_delegate_id":   previous.DelegateID,
		"delegate_id":            delegation.DelegateID,
		"previous_scope":         previous.Scope,
		"scope":                  delegation.Scope,
		"role":                   delegation.Role,
		"status":                 delegation.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal directory delegation reassign audit detail: %w", err)
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
	return mapDirectoryDelegationWriteError("create directory delegation", err)
}

func mapDirectoryDelegationUpdateError(err error) error {
	return mapDirectoryDelegationWriteError("update directory delegation", err)
}

func mapDirectoryDelegationWriteError(operation string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "idx_directory_delegations_active_grant":
			return fmt.Errorf("%w", ErrDelegationAlreadyExists)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

const listDelegationsBaseSQL = `
SELECT id::text,
       company_id::text,
       owner_kind,
       owner_id::text,
       delegate_kind,
       delegate_id::text,
       scope,
       role,
       status
FROM directory_delegations`

func buildListDelegationsQuery(req ListDelegationsRequest) (string, []any) {
	args := []any{req.CompanyID}
	conditions := []string{"company_id = $1::uuid"}
	if req.OwnerKind != "" {
		args = append(args, req.OwnerKind)
		conditions = append(conditions, fmt.Sprintf("owner_kind = $%d", len(args)))
	}
	if req.OwnerID != "" {
		args = append(args, req.OwnerID)
		conditions = append(conditions, fmt.Sprintf("owner_id = $%d::uuid", len(args)))
	}
	if req.DelegateKind != "" {
		args = append(args, req.DelegateKind)
		conditions = append(conditions, fmt.Sprintf("delegate_kind = $%d", len(args)))
	}
	if req.DelegateID != "" {
		args = append(args, req.DelegateID)
		conditions = append(conditions, fmt.Sprintf("delegate_id = $%d::uuid", len(args)))
	}
	if req.Scope != "" {
		args = append(args, req.Scope)
		conditions = append(conditions, fmt.Sprintf("scope = $%d", len(args)))
	}
	if req.Role != "" {
		args = append(args, req.Role)
		conditions = append(conditions, fmt.Sprintf("role = $%d", len(args)))
	}
	if req.ActiveOnly {
		conditions = append(conditions, "status = 'active'")
	}
	args = append(args, req.Limit)
	query := listDelegationsBaseSQL + "\nWHERE " + strings.Join(conditions, "\n  AND ") + fmt.Sprintf(`
ORDER BY updated_at DESC, id
LIMIT $%d`, len(args))
	return query, args
}

func (r *Repository) ListDelegations(ctx context.Context, req ListDelegationsRequest) ([]Delegation, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeListDelegationsRequest(req)
	if err != nil {
		return nil, err
	}
	query, args := buildListDelegationsQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
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

func (r *Repository) ensureDelegationReassignPrincipalsActive(ctx context.Context, req ReassignDelegationRequest) (string, error) {
	owner, err := r.ResolvePrincipal(ctx, ResolvePrincipalRequest{
		ID:         req.OwnerID,
		Kind:       req.OwnerKind,
		ActiveOnly: true,
	})
	if err != nil {
		return "", fmt.Errorf("resolve active delegation owner: %w", err)
	}
	delegate, err := r.ResolvePrincipal(ctx, ResolvePrincipalRequest{
		ID:         req.DelegateID,
		Kind:       req.DelegateKind,
		ActiveOnly: true,
	})
	if err != nil {
		return "", fmt.Errorf("resolve active delegation delegate: %w", err)
	}
	if owner.CompanyID == "" || owner.CompanyID != delegate.CompanyID {
		return "", fmt.Errorf("directory delegation principals must belong to the same company")
	}
	return owner.CompanyID, nil
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
