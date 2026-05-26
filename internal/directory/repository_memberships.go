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
