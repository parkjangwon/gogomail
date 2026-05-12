package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// TrackingPixel holds one row from message_tracking_pixels.
type TrackingPixel struct {
	PixelID        string
	MessageID      string
	SenderUserID   string
	RecipientEmail string
	CreatedAt      time.Time
}

// TrackingOpenEvent is an aggregated per-recipient open record returned to the caller.
type TrackingOpenEvent struct {
	RecipientEmail string
	OpenedAt       time.Time
	IP             string
	UserAgent      string
	OpenCount      int
}

// CreateTrackingPixels inserts multiple pixel rows in one transaction.
func (r *Repository) CreateTrackingPixels(ctx context.Context, pixels []TrackingPixel) error {
	if len(pixels) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create tracking pixels transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
INSERT INTO message_tracking_pixels (pixel_id, message_id, sender_user_id, recipient_email)
VALUES ($1, $2::uuid, $3::uuid, $4)
ON CONFLICT (pixel_id) DO NOTHING`

	for _, p := range pixels {
		if _, err := tx.ExecContext(ctx, query, p.PixelID, p.MessageID, p.SenderUserID, p.RecipientEmail); err != nil {
			return fmt.Errorf("insert tracking pixel for %q: %w", p.RecipientEmail, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create tracking pixels: %w", err)
	}
	return nil
}

// RecordTrackingOpen inserts a new open event for pixel_id.
// Returns nil if pixel_id is not found (silently ignored).
func (r *Repository) RecordTrackingOpen(ctx context.Context, pixelID, ip, userAgent string) error {
	const query = `
INSERT INTO message_tracking_events (pixel_id, ip, user_agent)
SELECT $1, $2, $3
WHERE EXISTS (SELECT 1 FROM message_tracking_pixels WHERE pixel_id = $1)`

	_, err := r.db.ExecContext(ctx, query, pixelID, ip, userAgent)
	if err != nil {
		return fmt.Errorf("record tracking open for pixel %q: %w", pixelID, err)
	}
	return nil
}

// ListTrackingEvents returns aggregated per-recipient open events for a message
// owned by senderUserID.
func (r *Repository) ListTrackingEvents(ctx context.Context, senderUserID, messageID string) ([]TrackingOpenEvent, error) {
	const query = `
SELECT
  p.recipient_email,
  MIN(e.opened_at) AS first_opened_at,
  (ARRAY_AGG(e.ip ORDER BY e.opened_at DESC))[1] AS last_ip,
  (ARRAY_AGG(e.user_agent ORDER BY e.opened_at DESC))[1] AS last_user_agent,
  COUNT(e.id) AS open_count
FROM message_tracking_pixels p
LEFT JOIN message_tracking_events e ON e.pixel_id = p.pixel_id
WHERE p.sender_user_id = $1::uuid
  AND p.message_id = $2::uuid
GROUP BY p.recipient_email
ORDER BY p.recipient_email`

	rows, err := r.db.QueryContext(ctx, query, senderUserID, messageID)
	if err != nil {
		return nil, fmt.Errorf("list tracking events: %w", err)
	}
	defer rows.Close()

	var events []TrackingOpenEvent
	for rows.Next() {
		var ev TrackingOpenEvent
		var openedAt sql.NullTime
		var ip, ua sql.NullString
		var count int64
		if err := rows.Scan(&ev.RecipientEmail, &openedAt, &ip, &ua, &count); err != nil {
			return nil, fmt.Errorf("scan tracking event row: %w", err)
		}
		if openedAt.Valid {
			ev.OpenedAt = openedAt.Time
		}
		if ip.Valid {
			ev.IP = ip.String
		}
		if ua.Valid {
			ev.UserAgent = ua.String
		}
		ev.OpenCount = int(count)
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tracking event rows: %w", err)
	}
	return events, nil
}
