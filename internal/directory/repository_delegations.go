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

func (r *Repository) GetDelegation(ctx context.Context, id string) (Delegation, error) {
	if r == nil || r.db == nil {
		return Delegation{}, fmt.Errorf("database handle is required")
	}
	id, err := NormalizePrincipalID(id)
	if err != nil {
		return Delegation{}, fmt.Errorf("delegation id: %w", err)
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
WHERE id = $1::uuid`
	var delegation Delegation
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
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
		return Delegation{}, fmt.Errorf("get directory delegation: %w", err)
	}
	return delegation, nil
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
