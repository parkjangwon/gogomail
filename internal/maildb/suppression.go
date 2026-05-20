package maildb

import (
	"context"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

const suppressedRecipientsSQL = `
WITH normalized AS (
  SELECT lower(btrim(email)) AS email, ordinality
  FROM unnest($2::text[]) WITH ORDINALITY AS requested(email, ordinality)
),
requested AS (
  SELECT email, min(ordinality) AS ordinality
  FROM normalized
  WHERE email <> ''
  GROUP BY email
)
SELECT requested.email
FROM requested
WHERE EXISTS (
  SELECT 1
  FROM suppression_list s
  WHERE lower(s.email) = requested.email
    AND COALESCE(s.domain_id, '00000000-0000-0000-0000-000000000000'::uuid)
      IN ($1::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
)
ORDER BY requested.ordinality`

func (r *Repository) SuppressedRecipients(ctx context.Context, domainID string, recipients []string) ([]string, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if len(recipients) == 0 {
		return nil, nil
	}
	domainID = strings.TrimSpace(domainID)
	rows, err := r.db.QueryContext(ctx, suppressedRecipientsSQL, domainID, pq.Array(recipients))
	if err != nil {
		return nil, fmt.Errorf("check suppression list: %w", err)
	}
	defer rows.Close()

	suppressed := make([]string, 0)
	for rows.Next() {
		var recipient string
		if err := rows.Scan(&recipient); err != nil {
			return nil, fmt.Errorf("scan suppressed recipient: %w", err)
		}
		suppressed = append(suppressed, recipient)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate suppressed recipients: %w", err)
	}
	return suppressed, nil
}
