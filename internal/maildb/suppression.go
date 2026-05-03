package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (r *Repository) SuppressedRecipients(ctx context.Context, domainID string, recipients []string) ([]string, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT 1
FROM suppression_list
WHERE lower(email) = lower($1)
  AND (domain_id = $2 OR domain_id IS NULL)
LIMIT 1`

	suppressed := make([]string, 0)
	seen := make(map[string]struct{}, len(recipients))
	for _, recipient := range recipients {
		normalized := strings.TrimSpace(strings.ToLower(recipient))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}

		var exists int
		if err := r.db.QueryRowContext(ctx, query, normalized, domainID).Scan(&exists); err == nil {
			suppressed = append(suppressed, normalized)
		} else if err != sql.ErrNoRows {
			return nil, fmt.Errorf("check suppression list for %q: %w", normalized, err)
		}
	}
	return suppressed, nil
}
