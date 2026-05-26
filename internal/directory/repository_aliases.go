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
