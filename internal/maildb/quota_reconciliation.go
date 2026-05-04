package maildb

import (
	"context"
	"database/sql"
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

type CorrectQuotaReconciliationRequest struct {
	Scope  string `json:"scope,omitempty"`
	ID     string `json:"id,omitempty"`
	DryRun bool   `json:"dry_run,omitempty"`
}

type QuotaCorrectionResult struct {
	DryRun    bool                      `json:"dry_run"`
	CheckedAt time.Time                 `json:"checked_at"`
	Corrected []QuotaReconciliationView `json:"corrected"`
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

func ValidateCorrectQuotaReconciliationRequest(req CorrectQuotaReconciliationRequest) (CorrectQuotaReconciliationRequest, error) {
	req.Scope = strings.ToLower(strings.TrimSpace(req.Scope))
	if req.Scope == "" {
		req.Scope = "all"
	}
	req.ID = strings.TrimSpace(req.ID)
	switch req.Scope {
	case "all":
		if req.ID != "" {
			return req, fmt.Errorf("id must be empty when scope is all")
		}
	case "company", "domain", "user":
		if req.ID == "" {
			return req, fmt.Errorf("id is required when scope is %s", req.Scope)
		}
	default:
		return req, fmt.Errorf("unsupported quota reconciliation scope %q", req.Scope)
	}
	return req, nil
}

func (r *Repository) CorrectQuotaReconciliation(ctx context.Context, req CorrectQuotaReconciliationRequest) (QuotaCorrectionResult, error) {
	if r.db == nil {
		return QuotaCorrectionResult{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateCorrectQuotaReconciliationRequest(req)
	if err != nil {
		return QuotaCorrectionResult{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return QuotaCorrectionResult{}, fmt.Errorf("begin quota reconciliation correction transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(172938225691027047) `); err != nil {
		return QuotaCorrectionResult{}, fmt.Errorf("lock quota reconciliation correction: %w", err)
	}
	if err := lockQuotaCorrectionScope(ctx, tx, req); err != nil {
		return QuotaCorrectionResult{}, err
	}

	before, err := listQuotaReconciliationTx(ctx, tx, req)
	if err != nil {
		return QuotaCorrectionResult{}, err
	}
	result := QuotaCorrectionResult{
		DryRun:    req.DryRun,
		CheckedAt: time.Now().UTC(),
		Corrected: driftedQuotaViews(before),
	}
	if req.DryRun {
		return result, tx.Commit()
	}

	if err := applyQuotaCorrection(ctx, tx, req); err != nil {
		return QuotaCorrectionResult{}, err
	}
	after, err := listQuotaReconciliationTx(ctx, tx, req)
	if err != nil {
		return QuotaCorrectionResult{}, err
	}
	result.Corrected = driftedQuotaViews(after)
	if err := tx.Commit(); err != nil {
		return QuotaCorrectionResult{}, fmt.Errorf("commit quota reconciliation correction: %w", err)
	}
	return result, nil
}

func lockQuotaCorrectionScope(ctx context.Context, tx *sql.Tx, req CorrectQuotaReconciliationRequest) error {
	switch req.Scope {
	case "all":
		if _, err := tx.ExecContext(ctx, `SELECT id FROM companies FOR UPDATE`); err != nil {
			return fmt.Errorf("lock company quota rows: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `SELECT id FROM domains FOR UPDATE`); err != nil {
			return fmt.Errorf("lock domain quota rows: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `SELECT id FROM users FOR UPDATE`); err != nil {
			return fmt.Errorf("lock user quota rows: %w", err)
		}
	case "company":
		if _, err := tx.ExecContext(ctx, `SELECT id FROM companies WHERE id = $1 FOR UPDATE`, req.ID); err != nil {
			return fmt.Errorf("lock company quota row: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `SELECT id FROM domains WHERE company_id = $1 FOR UPDATE`, req.ID); err != nil {
			return fmt.Errorf("lock domain quota rows: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `SELECT u.id FROM users u JOIN domains d ON d.id = u.domain_id WHERE d.company_id = $1 FOR UPDATE OF u`, req.ID); err != nil {
			return fmt.Errorf("lock user quota rows: %w", err)
		}
	case "domain":
		if _, err := tx.ExecContext(ctx, `SELECT c.id FROM domains d JOIN companies c ON c.id = d.company_id WHERE d.id = $1 FOR UPDATE OF c`, req.ID); err != nil {
			return fmt.Errorf("lock parent company quota row: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `SELECT id FROM domains WHERE id = $1 FOR UPDATE`, req.ID); err != nil {
			return fmt.Errorf("lock domain quota row: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `SELECT id FROM users WHERE domain_id = $1 FOR UPDATE`, req.ID); err != nil {
			return fmt.Errorf("lock user quota rows: %w", err)
		}
	case "user":
		if _, err := tx.ExecContext(ctx, `SELECT u.id, d.id, c.id FROM users u JOIN domains d ON d.id = u.domain_id JOIN companies c ON c.id = d.company_id WHERE u.id = $1 FOR UPDATE OF u, d, c`, req.ID); err != nil {
			return fmt.Errorf("lock user quota hierarchy: %w", err)
		}
	}
	return nil
}

func listQuotaReconciliationTx(ctx context.Context, tx *sql.Tx, req CorrectQuotaReconciliationRequest) ([]QuotaReconciliationView, error) {
	rows, err := tx.QueryContext(ctx, quotaReconciliationFilteredQuery(req.Scope), req.ID)
	if err != nil {
		return nil, fmt.Errorf("list scoped quota reconciliation: %w", err)
	}
	defer rows.Close()
	return scanQuotaReconciliationRows(rows, time.Now().UTC())
}

func scanQuotaReconciliationRows(rows *sql.Rows, checkedAt time.Time) ([]QuotaReconciliationView, error) {
	var views []QuotaReconciliationView
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

func driftedQuotaViews(views []QuotaReconciliationView) []QuotaReconciliationView {
	out := make([]QuotaReconciliationView, 0, len(views))
	for _, view := range views {
		if !view.InSync {
			out = append(out, view)
		}
	}
	return out
}

func applyQuotaCorrection(ctx context.Context, tx *sql.Tx, req CorrectQuotaReconciliationRequest) error {
	if _, err := tx.ExecContext(ctx, quotaCorrectionUpdateUsersSQL(req.Scope), req.ID); err != nil {
		return fmt.Errorf("correct user quota ledgers: %w", err)
	}
	if _, err := tx.ExecContext(ctx, quotaCorrectionUpdateDomainsSQL(req.Scope), req.ID); err != nil {
		return fmt.Errorf("correct domain quota ledgers: %w", err)
	}
	if _, err := tx.ExecContext(ctx, quotaCorrectionUpdateCompaniesSQL(req.Scope), req.ID); err != nil {
		return fmt.Errorf("correct company quota ledgers: %w", err)
	}
	return nil
}

func quotaReconciliationFilteredQuery(scope string) string {
	return quotaActualCTE + `
SELECT scope, id, domain_id, name, ledger_used, actual_used
FROM quota_reconciliation_all
WHERE ($1 = '' OR
  ($1 = id AND scope = '` + scope + `') OR
  ('` + scope + `' = 'company' AND $1 = company_id) OR
  ('` + scope + `' = 'domain' AND $1 = domain_id)
)
ORDER BY ABS(ledger_used - actual_used) DESC, scope, name`
}

const quotaActualCTE = `
WITH user_actual AS (
  SELECT
    u.id,
    d.id AS domain_id,
    d.company_id,
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
),
quota_reconciliation_all AS (
  SELECT 'company' AS scope, id::text AS id, '' AS domain_id, id::text AS company_id, name, ledger_used, actual_used FROM company_actual
  UNION ALL
  SELECT 'domain' AS scope, id::text AS id, id::text AS domain_id, company_id::text AS company_id, name, ledger_used, actual_used FROM domain_actual
  UNION ALL
  SELECT 'user' AS scope, id::text AS id, domain_id::text AS domain_id, company_id::text AS company_id, name, ledger_used, actual_used FROM user_actual
)`

func quotaCorrectionUpdateUsersSQL(scope string) string {
	return quotaActualCTE + `
UPDATE users u
SET quota_used = user_actual.actual_used,
    updated_at = now()
FROM user_actual
WHERE u.id = user_actual.id
  AND ($1 = '' OR
    ('` + scope + `' = 'user' AND u.id::text = $1) OR
    ('` + scope + `' = 'domain' AND user_actual.domain_id::text = $1) OR
    ('` + scope + `' = 'company' AND user_actual.company_id::text = $1)
  )`
}

func quotaCorrectionUpdateDomainsSQL(scope string) string {
	return quotaActualCTE + `
UPDATE domains d
SET quota_used = domain_actual.actual_used,
    updated_at = now()
FROM domain_actual
WHERE d.id = domain_actual.id
  AND ($1 = '' OR
    ('` + scope + `' = 'domain' AND d.id::text = $1) OR
    ('` + scope + `' = 'user' AND EXISTS (SELECT 1 FROM users u WHERE u.id::text = $1 AND u.domain_id = d.id)) OR
    ('` + scope + `' = 'company' AND d.company_id::text = $1)
  )`
}

func quotaCorrectionUpdateCompaniesSQL(scope string) string {
	return quotaActualCTE + `
UPDATE companies c
SET quota_used = company_actual.actual_used,
    updated_at = now()
FROM company_actual
WHERE c.id = company_actual.id
  AND ($1 = '' OR
    ('` + scope + `' = 'company' AND c.id::text = $1) OR
    ('` + scope + `' = 'domain' AND EXISTS (SELECT 1 FROM domains d WHERE d.id::text = $1 AND d.company_id = c.id)) OR
    ('` + scope + `' = 'user' AND EXISTS (
      SELECT 1
      FROM users u
      JOIN domains d ON d.id = u.domain_id
      WHERE u.id::text = $1 AND d.company_id = c.id
    ))
  )`
}
