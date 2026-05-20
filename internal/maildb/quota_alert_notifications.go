package maildb

import (
	"context"
	"fmt"
)

type PendingQuotaAlertEmail struct {
	ID    string
	Email string
	Pct   int
}

func (r *Repository) ListPendingUserQuotaAlertEmails(ctx context.Context, limit int) ([]PendingQuotaAlertEmail, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if limit <= 0 {
		limit = 100
	}
	const query = `
SELECT
  qa.id::text,
  COALESCE(primary_addr.address, u.username || '@' || d.name) AS email,
  CEIL(qa.usage_ratio * 100)::int AS pct
FROM quota_alerts qa
JOIN users u ON u.id = qa.user_id
JOIN domains d ON d.id = u.domain_id
LEFT JOIN LATERAL (
  SELECT address
  FROM user_addresses
  WHERE user_id = u.id
  ORDER BY is_primary DESC, created_at ASC
  LIMIT 1
) primary_addr ON true
WHERE qa.scope = 'user'
  AND qa.notified_at IS NULL
ORDER BY qa.created_at ASC, qa.id ASC
LIMIT $1`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending quota alert emails: %w", err)
	}
	defer rows.Close()

	var alerts []PendingQuotaAlertEmail
	for rows.Next() {
		var alert PendingQuotaAlertEmail
		if err := rows.Scan(&alert.ID, &alert.Email, &alert.Pct); err != nil {
			return nil, fmt.Errorf("scan pending quota alert email: %w", err)
		}
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending quota alert emails: %w", err)
	}
	return alerts, nil
}

func (r *Repository) MarkQuotaAlertNotified(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if id == "" {
		return fmt.Errorf("quota alert id is required")
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE quota_alerts SET notified_at = now(), updated_at = now() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("mark quota alert notified: %w", err)
	}
	return nil
}
