package deltasync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PostgresCursorStore persists delta-sync cursors in PostgreSQL.
type PostgresCursorStore struct {
	db *sql.DB
}

// NewPostgresCursorStore creates a Postgres-backed CursorStore.
func NewPostgresCursorStore(db *sql.DB) *PostgresCursorStore {
	return &PostgresCursorStore{db: db}
}

// Save upserts a cursor row, updating version and updated_at on conflict.
func (s *PostgresCursorStore) Save(ctx context.Context, cursor *Cursor) error {
	if s.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id := strings.TrimSpace(cursor.ID)
	deviceID := strings.TrimSpace(cursor.DeviceID)
	userID := strings.TrimSpace(cursor.UserID)
	mailboxID := strings.TrimSpace(cursor.MailboxID)
	if id == "" || deviceID == "" || mailboxID == "" {
		return fmt.Errorf("cursor id, device_id, and mailbox_id are required")
	}

	const query = `
INSERT INTO device_sync_cursors (id, device_id, user_id, mailbox_id, version, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (device_id, mailbox_id)
DO UPDATE SET
  version    = EXCLUDED.version,
  updated_at = EXCLUDED.updated_at`

	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, query,
		id, deviceID, userID, mailboxID, cursor.Version, now, now,
	); err != nil {
		return fmt.Errorf("save cursor: %w", err)
	}
	return nil
}

// Get retrieves the cursor for a device+mailbox pair.
func (s *PostgresCursorStore) Get(ctx context.Context, deviceID, mailboxID string) (*Cursor, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	deviceID = strings.TrimSpace(deviceID)
	mailboxID = strings.TrimSpace(mailboxID)
	if deviceID == "" || mailboxID == "" {
		return nil, fmt.Errorf("device_id and mailbox_id are required")
	}

	const query = `
SELECT id, device_id, user_id, mailbox_id, version, created_at
FROM device_sync_cursors
WHERE device_id = $1 AND mailbox_id = $2`

	var c Cursor
	err := s.db.QueryRowContext(ctx, query, deviceID, mailboxID).Scan(
		&c.ID, &c.DeviceID, &c.UserID, &c.MailboxID, &c.Version, &c.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("cursor not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get cursor: %w", err)
	}
	return &c, nil
}

// ListByMailbox returns all cursors for a mailbox, ordered by device_id.
func (s *PostgresCursorStore) ListByMailbox(ctx context.Context, mailboxID string) ([]*Cursor, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	mailboxID = strings.TrimSpace(mailboxID)
	if mailboxID == "" {
		return nil, fmt.Errorf("mailbox_id is required")
	}

	const query = `
SELECT id, device_id, user_id, mailbox_id, version, created_at
FROM device_sync_cursors
WHERE mailbox_id = $1
ORDER BY device_id`

	rows, err := s.db.QueryContext(ctx, query, mailboxID)
	if err != nil {
		return nil, fmt.Errorf("list cursors by mailbox: %w", err)
	}
	defer rows.Close()

	var result []*Cursor
	for rows.Next() {
		var c Cursor
		if err := rows.Scan(&c.ID, &c.DeviceID, &c.UserID, &c.MailboxID, &c.Version, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan cursor: %w", err)
		}
		result = append(result, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cursors: %w", err)
	}
	return result, nil
}

// Delete removes a cursor by ID.
func (s *PostgresCursorStore) Delete(ctx context.Context, id string) error {
	if s.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id is required")
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM device_sync_cursors WHERE id = $1`, id,
	); err != nil {
		return fmt.Errorf("delete cursor: %w", err)
	}
	return nil
}
