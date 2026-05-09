package maildb

import (
	"context"
	"fmt"
	"time"
)

// CountStuckScheduledMail returns the number of batch outbox entries that are
// past their available_at time but still pending after stuckAfter has elapsed.
// A non-zero result indicates the outbox worker may be lagging or stopped.
func (r *Repository) CountStuckScheduledMail(ctx context.Context, stuckAfter time.Duration) (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT count(*)
FROM outbox
WHERE topic LIKE 'mail.outbound.batch%'
  AND status = 'pending'
  AND available_at <= now()
  AND created_at  <= now() - $1::interval`
	var n int64
	err := r.db.QueryRowContext(ctx, query, stuckAfter.String()).Scan(&n)
	return n, err
}
