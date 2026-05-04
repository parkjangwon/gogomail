package maildb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gogomail/gogomail/internal/mail"
)

// checkAndIncrementUserQuota checks whether userID has room for size bytes and
// atomically increments quota_used when the limit permits it.  It must be
// called inside an open transaction; the FOR UPDATE lock prevents concurrent
// over-quota writes.  Returns mail.ErrMailboxFull when the quota would be
// exceeded.
func checkAndIncrementUserQuota(ctx context.Context, tx *sql.Tx, userID string, size int64) error {
	if size <= 0 {
		return nil
	}

	const selectQ = `
SELECT quota_used, COALESCE(quota_limit, 0)
FROM users
WHERE id = $1
FOR UPDATE`

	var used, limit int64
	if err := tx.QueryRowContext(ctx, selectQ, userID).Scan(&used, &limit); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("user %q not found for quota check", userID)
		}
		return fmt.Errorf("read user quota: %w", err)
	}
	if limit > 0 && used+size > limit {
		return fmt.Errorf("%w: used %d, limit %d, message %d bytes", mail.ErrMailboxFull, used, limit, size)
	}

	const updateQ = `
UPDATE users
SET quota_used = quota_used + $2,
    updated_at = now()
WHERE id = $1`

	if _, err := tx.ExecContext(ctx, updateQ, userID, size); err != nil {
		return fmt.Errorf("increment user quota: %w", err)
	}
	return nil
}

// decrementUserQuota subtracts size bytes from the user's quota_used, clamping
// at zero so stale accounting cannot produce a negative balance.
func decrementUserQuota(ctx context.Context, tx *sql.Tx, userID string, size int64) error {
	if size <= 0 {
		return nil
	}

	const q = `
UPDATE users
SET quota_used = GREATEST(0, quota_used - $2),
    updated_at = now()
WHERE id = $1`

	if _, err := tx.ExecContext(ctx, q, userID, size); err != nil {
		return fmt.Errorf("decrement user quota: %w", err)
	}
	return nil
}
