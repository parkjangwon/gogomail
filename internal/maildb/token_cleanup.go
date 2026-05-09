package maildb

import (
	"context"
	"fmt"
	"time"
)

// PruneExpiredAttachmentShareLinks deletes attachment share links whose
// expires_at is before cutoff, returning the number of rows deleted.
func (r *Repository) PruneExpiredAttachmentShareLinks(ctx context.Context, cutoff time.Time) (int, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	if cutoff.IsZero() {
		return 0, fmt.Errorf("cutoff time is required")
	}
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM attachment_share_links WHERE expires_at < $1`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("prune expired attachment share links: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// PruneExpiredDriveShareLinks deletes drive share links whose expires_at is
// before cutoff, returning the number of rows deleted.
func (r *Repository) PruneExpiredDriveShareLinks(ctx context.Context, cutoff time.Time) (int, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	if cutoff.IsZero() {
		return 0, fmt.Errorf("cutoff time is required")
	}
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM drive_share_links WHERE expires_at < $1`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("prune expired drive share links: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}
