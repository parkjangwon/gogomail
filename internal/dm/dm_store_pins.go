package dm

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// PinMessage pins a message in a room for quick access.
// It verifies that the message belongs to the room and that the principal is a
// room member. Pinning a message that is already pinned is idempotent and does
// not return an error. The optional note is a human-readable reason stored
// alongside the pin (never message plaintext).
func (s *PostgresStore) PinMessage(ctx context.Context, principal Principal, roomID string, messageID string, note string) error {
	if err := s.requireDB(); err != nil {
		return err
	}
	roomID = strings.TrimSpace(roomID)
	messageID = strings.TrimSpace(messageID)
	if roomID == "" || messageID == "" {
		return fmt.Errorf("%w: room_id and message_id are required", ErrInvalid)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Verify the principal is a member of the room within their tenant scope.
	if err := ensureParticipantTx(ctx, tx, principal, roomID); err != nil {
		return err
	}

	// Verify the target message belongs to this room and is not soft-deleted or
	// a system message (system messages are not user content and cannot be pinned).
	if err := ensurePinnableMessageTx(ctx, tx, roomID, messageID); err != nil {
		return err
	}

	const insert = `
INSERT INTO dm_message_pins (room_id, message_id, pinned_by_user_id, note)
VALUES ($1, $2, $3, NULLIF($4, ''))
ON CONFLICT (room_id, message_id) DO NOTHING`
	if _, err := tx.ExecContext(ctx, insert, roomID, messageID, principal.UserID, strings.TrimSpace(note)); err != nil {
		return err
	}
	return tx.Commit()
}

// UnpinMessage removes a pin. It verifies the principal is a room member.
// Unpinning a message that is not pinned is a no-op success.
func (s *PostgresStore) UnpinMessage(ctx context.Context, principal Principal, roomID string, messageID string) error {
	if err := s.requireDB(); err != nil {
		return err
	}
	roomID = strings.TrimSpace(roomID)
	messageID = strings.TrimSpace(messageID)
	if roomID == "" || messageID == "" {
		return fmt.Errorf("%w: room_id and message_id are required", ErrInvalid)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := ensureParticipantTx(ctx, tx, principal, roomID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dm_message_pins WHERE room_id = $1 AND message_id = $2`, roomID, messageID); err != nil {
		return err
	}
	return tx.Commit()
}

// ListPinnedMessages returns pinned messages for a room, newest pins first.
// It verifies the principal is a room member. Each returned record carries the
// encrypted message body (for later decryption by the service) plus pin
// metadata (who pinned it, when, and the optional note). Soft-deleted messages
// remain listed so a pin never silently disappears; the service renders their
// body as the localized deleted-message placeholder.
func (s *PostgresStore) ListPinnedMessages(ctx context.Context, principal Principal, roomID string, limit int, offset int) ([]PinnedRecord, error) {
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return nil, fmt.Errorf("%w: room_id is required", ErrInvalid)
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	// The join to dm_participants enforces membership; combined with the
	// company/domain predicates this maintains tenant isolation.
	query := pinnedMessageSelectSQL + `
WHERE pin.room_id = $1
  AND r.company_id = $2
  AND r.domain_id = $3
  AND p.user_id = $4
ORDER BY pin.pinned_at DESC, pin.id DESC
LIMIT $5 OFFSET $6`
	rows, err := s.db.QueryContext(ctx, query, roomID, principal.CompanyID, principal.DomainID, principal.UserID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PinnedRecord
	for rows.Next() {
		var pr PinnedRecord
		record, err := scanMessageRecordWithTail(rows, &pr.PinnedByUserID, &pr.PinnedAt, &pr.Note)
		if err != nil {
			return nil, err
		}
		pr.Record = record
		out = append(out, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ensurePinnableMessageTx returns ErrNotFound if the message does not exist in
// the room, and ErrInvalid if it is a system message. Soft-deleted messages are
// still pinnable so a previously-pinned message that gets deleted keeps a
// consistent shape; new pins on deleted messages are also allowed because the
// pin only references the message id.
func ensurePinnableMessageTx(ctx context.Context, tx *sql.Tx, roomID string, messageID string) error {
	const query = `SELECT message_type FROM dm_messages WHERE id = $1 AND room_id = $2`
	var messageType string
	if err := tx.QueryRowContext(ctx, query, messageID, roomID).Scan(&messageType); err != nil {
		return mapNoRows(err)
	}
	if messageType == MessageTypeSystem {
		return fmt.Errorf("%w: system messages cannot be pinned", ErrInvalid)
	}
	return nil
}

// pinnedMessageSelectSQL selects the encrypted message columns (matching the
// scanMessageRecord layout) followed by the three pin-metadata tail columns
// consumed by scanMessageRecordWithTail: pinned_by_user_id, pinned_at, note.
const pinnedMessageSelectSQL = `
SELECT m.id::text, m.room_id::text, COALESCE(m.sender_id::text, ''), m.message_type, m.body,
  COALESCE(m.attachment_storage_path, NULL), COALESCE(m.attachment_name, ''), COALESCE(m.attachment_size, 0),
  COALESCE(m.attachment_mime_type, ''), COALESCE(m.drive_file_id::text, ''),
  m.created_at, m.edited_at, m.deleted_at,
  0::int AS read_count,
  '[]'::text AS reactions,
  pin.pinned_by_user_id::text, pin.pinned_at, COALESCE(pin.note, '')
FROM dm_message_pins pin
JOIN dm_messages m ON m.id = pin.message_id
JOIN dm_rooms r ON r.id = pin.room_id
JOIN dm_participants p ON p.room_id = r.id`
