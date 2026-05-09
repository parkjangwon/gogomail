package maildb

import (
	"context"
	"fmt"
)

// ScanAndRecordQuotaAlerts scans all quota-enabled entities (users, domains,
// companies) and inserts quota_alerts rows for those whose usage ratio exceeds
// a threshold and that have not been alerted with the same alert type in the
// last 24 hours. Returns the number of alerts recorded.
//
// defaultWarning and defaultCritical are applied when no per-entity threshold
// row exists in quota_alert_thresholds. Both must be in (0, 1] and
// defaultWarning must be <= defaultCritical.
func (r *Repository) ScanAndRecordQuotaAlerts(ctx context.Context, defaultWarning, defaultCritical float64) (int, error) {
	if defaultWarning <= 0 || defaultWarning > 1 {
		return 0, fmt.Errorf("defaultWarning must be in (0, 1], got %v", defaultWarning)
	}
	if defaultCritical <= 0 || defaultCritical > 1 {
		return 0, fmt.Errorf("defaultCritical must be in (0, 1], got %v", defaultCritical)
	}
	if defaultWarning > defaultCritical {
		return 0, fmt.Errorf("defaultWarning (%v) must be <= defaultCritical (%v)", defaultWarning, defaultCritical)
	}
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}

	// Single INSERT...SELECT using CTEs to find entities over threshold and
	// record alerts, skipping entities already alerted in the last 24 hours.
	const query = `
WITH entity_usage AS (
  SELECT
    c.id::text AS entity_id,
    c.id::text AS company_id,
    ''         AS domain_id,
    ''         AS user_id,
    'company'  AS scope,
    c.quota_used,
    c.quota_limit,
    c.quota_used::double precision / c.quota_limit AS ratio
  FROM companies c
  WHERE c.quota_limit > 0 AND c.quota_used > 0

  UNION ALL

  SELECT
    d.id::text,
    d.company_id::text,
    d.id::text,
    '',
    'domain',
    d.quota_used,
    d.quota_limit,
    d.quota_used::double precision / d.quota_limit
  FROM domains d
  WHERE d.quota_limit > 0 AND d.quota_used > 0

  UNION ALL

  SELECT
    u.id::text,
    d.company_id::text,
    u.domain_id::text,
    u.id::text,
    'user',
    u.quota_used,
    u.quota_limit,
    u.quota_used::double precision / u.quota_limit
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  WHERE u.quota_limit > 0 AND u.quota_used > 0
),
thresholds AS (
  SELECT
    scope,
    COALESCE(scope_id::text, '') AS scope_id,
    company_id::text             AS company_id,
    warning_ratio,
    critical_ratio
  FROM quota_alert_thresholds
),
candidates AS (
  SELECT
    eu.*,
    COALESCE(t.warning_ratio,  $1) AS eff_warning,
    COALESCE(t.critical_ratio, $2) AS eff_critical,
    CASE
      WHEN eu.ratio >= 1.0 THEN 'exhausted'
      WHEN eu.ratio >= COALESCE(t.critical_ratio, $2) THEN 'critical'
      ELSE 'warning'
    END AS alert_type
  FROM entity_usage eu
  LEFT JOIN thresholds t
    ON  t.scope      = eu.scope
    AND t.company_id = eu.company_id
    AND (t.scope_id  = eu.entity_id OR t.scope_id = '')
  WHERE eu.ratio >= COALESCE(t.warning_ratio, $1)
),
inserted AS (
  INSERT INTO quota_alerts
    (company_id, domain_id, user_id, scope, alert_type, quota_used, quota_limit, usage_ratio, event_id)
  SELECT
    c.company_id::uuid,
    NULLIF(c.domain_id, '')::uuid,
    NULLIF(c.user_id,   '')::uuid,
    c.scope,
    c.alert_type,
    c.quota_used,
    c.quota_limit,
    c.ratio,
    gen_random_uuid()
  FROM candidates c
  WHERE NOT EXISTS (
    SELECT 1 FROM quota_alerts qa
    WHERE qa.scope      = c.scope
      AND qa.alert_type = c.alert_type
      AND qa.company_id::text = c.company_id
      AND CASE c.scope
            WHEN 'user'    THEN qa.user_id::text   = c.entity_id
            WHEN 'domain'  THEN qa.domain_id::text = c.entity_id
            WHEN 'company' THEN qa.company_id::text = c.entity_id
            ELSE false
          END
      AND qa.created_at >= now() - interval '24 hours'
  )
  RETURNING id
)
SELECT COUNT(*) FROM inserted`

	var n int
	if err := r.db.QueryRowContext(ctx, query, defaultWarning, defaultCritical).Scan(&n); err != nil {
		return 0, fmt.Errorf("scan and record quota alerts: %w", err)
	}
	return n, nil
}
