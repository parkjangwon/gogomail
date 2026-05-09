package maildb

import (
	"context"
	"fmt"
	"time"
)

// PruneExpiredTOTPCodes deletes used TOTP codes older than cutoff from
// totp_used_codes, returning the number of rows deleted.
func (r *Repository) PruneExpiredTOTPCodes(ctx context.Context, cutoff time.Time) (int, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	if cutoff.IsZero() {
		return 0, fmt.Errorf("cutoff time is required")
	}
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM totp_used_codes WHERE used_at < $1`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("prune expired TOTP codes: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}
