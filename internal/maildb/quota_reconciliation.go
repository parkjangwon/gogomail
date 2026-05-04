package maildb

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type QuotaReconciliationView struct {
	Scope      string    `json:"scope"`
	ID         string    `json:"id"`
	DomainID   string    `json:"domain_id,omitempty"`
	Name       string    `json:"name"`
	LedgerUsed int64     `json:"ledger_used"`
	ActualUsed int64     `json:"actual_used"`
	Delta      int64     `json:"delta"`
	InSync     bool      `json:"in_sync"`
	CheckedAt  time.Time `json:"checked_at"`
}

func (r *Repository) ListQuotaReconciliation(ctx context.Context, limit int) ([]QuotaReconciliationView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
WITH user_actual AS (
  SELECT
    u.id,
    u.domain_id,
    u.username || '@' || d.name_ace AS name,
    u.quota_used AS ledger_used,
    COALESCE(messages.bytes, 0) + COALESCE(attachments.bytes, 0) AS actual_used
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  LEFT JOIN (
    SELECT user_id, SUM(size) AS bytes
    FROM messages
    WHERE status = 'active'
    GROUP BY user_id
  ) messages ON messages.user_id = u.id
  LEFT JOIN (
    SELECT user_id, SUM(size) AS bytes
    FROM attachments
    WHERE user_id IS NOT NULL
      AND status IN ('uploading', 'stored')
    GROUP BY user_id
  ) attachments ON attachments.user_id = u.id
),
domain_actual AS (
  SELECT
    d.id,
    d.company_id,
    d.name,
    d.quota_used AS ledger_used,
    COALESCE(SUM(user_actual.actual_used), 0) AS actual_used
  FROM domains d
  LEFT JOIN user_actual ON user_actual.domain_id = d.id
  GROUP BY d.id, d.company_id, d.name, d.quota_used
),
company_actual AS (
  SELECT
    c.id,
    c.name,
    c.quota_used AS ledger_used,
    COALESCE(SUM(domain_actual.actual_used), 0) AS actual_used
  FROM companies c
  LEFT JOIN domain_actual ON domain_actual.company_id = c.id
  GROUP BY c.id, c.name, c.quota_used
)
SELECT scope, id, domain_id, name, ledger_used, actual_used
FROM (
  SELECT
    'company' AS scope,
    id::text AS id,
    '' AS domain_id,
    name,
    ledger_used,
    actual_used
  FROM company_actual
  UNION ALL
  SELECT
    'domain' AS scope,
    id::text AS id,
    id::text AS domain_id,
    name,
    ledger_used,
    actual_used
  FROM domain_actual
  UNION ALL
  SELECT
    'user' AS scope,
    id::text AS id,
    domain_id::text AS domain_id,
    name,
    ledger_used,
    actual_used
  FROM user_actual
) usage
ORDER BY ABS(ledger_used - actual_used) DESC, scope, name
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list quota reconciliation: %w", err)
	}
	defer rows.Close()

	checkedAt := time.Now().UTC()
	views := make([]QuotaReconciliationView, 0, limit)
	for rows.Next() {
		var view QuotaReconciliationView
		if err := rows.Scan(
			&view.Scope,
			&view.ID,
			&view.DomainID,
			&view.Name,
			&view.LedgerUsed,
			&view.ActualUsed,
		); err != nil {
			return nil, fmt.Errorf("scan quota reconciliation: %w", err)
		}
		view.Scope = strings.TrimSpace(view.Scope)
		view.Delta = view.LedgerUsed - view.ActualUsed
		view.InSync = view.Delta == 0
		view.CheckedAt = checkedAt
		views = append(views, view)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quota reconciliation: %w", err)
	}
	return views, nil
}
