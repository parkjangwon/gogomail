package maildb

import (
	"context"
	"fmt"
	"strings"
)

type AutoPurgePolicy struct {
	CompanyID                 string
	DeletedItemsRetentionDays int
	AuditLogRetentionDays     int
}

type AutoPurgeResult struct {
	CompaniesScanned int
	MessagesDeleted  int64
	AuditLogsDeleted int64
}

func (r *Repository) GetCompaniesWithAutoPurge(ctx context.Context) ([]AutoPurgePolicy, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT
  scope_id::text,
  CASE
    WHEN value->>'deleted_items_retention_days' ~ '^[0-9]+$' THEN (value->>'deleted_items_retention_days')::int
    ELSE 30
  END,
  CASE
    WHEN value->>'audit_log_retention_days' ~ '^[0-9]+$' THEN (value->>'audit_log_retention_days')::int
    ELSE 365
  END
FROM runtime_config
WHERE scope_type = 'company'
  AND key = 'retention_policy'
  AND COALESCE((value->>'auto_purge_enabled')::boolean, false) = true
ORDER BY updated_at ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list autopurge companies: %w", err)
	}
	defer rows.Close()

	var policies []AutoPurgePolicy
	for rows.Next() {
		var policy AutoPurgePolicy
		if err := rows.Scan(&policy.CompanyID, &policy.DeletedItemsRetentionDays, &policy.AuditLogRetentionDays); err != nil {
			return nil, fmt.Errorf("scan autopurge policy: %w", err)
		}
		policies = append(policies, policy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate autopurge policies: %w", err)
	}
	return policies, nil
}

func (r *Repository) RunAutoPurge(ctx context.Context, limit int) (AutoPurgeResult, error) {
	policies, err := r.GetCompaniesWithAutoPurge(ctx)
	if err != nil {
		return AutoPurgeResult{}, err
	}
	result := AutoPurgeResult{CompaniesScanned: len(policies)}
	for _, policy := range policies {
		deleted, err := r.PurgeExpiredTrashMessages(ctx, policy.CompanyID, policy.DeletedItemsRetentionDays, limit)
		if err != nil {
			return result, err
		}
		result.MessagesDeleted += deleted
		auditDeleted, err := r.PurgeExpiredAuditLogs(ctx, policy.CompanyID, policy.AuditLogRetentionDays, limit)
		if err != nil {
			return result, err
		}
		result.AuditLogsDeleted += auditDeleted
	}
	return result, nil
}

func (r *Repository) PurgeExpiredTrashMessages(ctx context.Context, companyID string, retentionDays, limit int) (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return 0, fmt.Errorf("company id is required")
	}
	if retentionDays <= 0 {
		return 0, nil
	}
	if limit <= 0 {
		limit = 1000
	}
	const query = `
WITH candidates AS (
  SELECT m.id
  FROM messages m
  JOIN domains d ON d.id = m.domain_id
  JOIN folders f ON f.id = m.folder_id
  WHERE d.company_id = $1::uuid
    AND m.legal_hold = false
    AND (m.status = 'deleted' OR lower(COALESCE(f.system_type, '')) = 'trash')
    AND COALESCE(m.deleted_at, m.updated_at, m.created_at) < now() - ($2 * interval '1 day')
  ORDER BY COALESCE(m.deleted_at, m.updated_at, m.created_at) ASC, m.id ASC
  LIMIT $3
)
DELETE FROM messages m
USING candidates c
WHERE m.id = c.id`
	res, err := r.db.ExecContext(ctx, query, companyID, retentionDays, limit)
	if err != nil {
		return 0, fmt.Errorf("purge expired trash messages: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("inspect purged trash messages: %w", err)
	}
	return n, nil
}

func (r *Repository) PurgeExpiredAuditLogs(ctx context.Context, companyID string, retentionDays, limit int) (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return 0, fmt.Errorf("company id is required")
	}
	if retentionDays <= 0 {
		return 0, nil
	}
	if limit <= 0 {
		limit = 1000
	}
	const query = `
WITH candidates AS (
  SELECT id
  FROM audit_logs
  WHERE company_id = $1::uuid
    AND created_at < now() - ($2 * interval '1 day')
  ORDER BY created_at ASC, id ASC
  LIMIT $3
)
DELETE FROM audit_logs a
USING candidates c
WHERE a.id = c.id`
	res, err := r.db.ExecContext(ctx, query, companyID, retentionDays, limit)
	if err != nil {
		return 0, fmt.Errorf("purge expired audit logs: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("inspect purged audit logs: %w", err)
	}
	return n, nil
}
