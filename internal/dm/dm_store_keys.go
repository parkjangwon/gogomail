package dm

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (s *PostgresStore) CreateInvite(ctx context.Context, principal Principal, roomID string, expiresAt time.Time) (Invite, error) {
	if err := s.requireDB(); err != nil {
		return Invite{}, err
	}
	const query = `
INSERT INTO dm_invites (room_id, created_by, expires_at)
SELECT r.id, $1, $5
FROM dm_rooms r
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $1
WHERE r.id = $2
  AND r.company_id = $3
  AND r.domain_id = $4
  AND r.room_type = 'group'
  AND r.visibility = 'private'
  AND r.owner_id = $1
RETURNING id::text, room_id::text, expires_at`
	var invite Invite
	if err := s.db.QueryRowContext(ctx, query, principal.UserID, roomID, principal.CompanyID, principal.DomainID, expiresAt).Scan(&invite.Token, &invite.RoomID, &invite.ExpiresAt); err != nil {
		return Invite{}, mapNoRows(err)
	}
	return invite, nil
}

func (s *PostgresStore) RoomKeyForInvite(ctx context.Context, principal Principal, token string) (string, []byte, error) {
	if err := s.requireDB(); err != nil {
		return "", nil, err
	}
	const query = `
SELECT r.id::text, k.key_ciphertext, EXISTS (
  SELECT 1 FROM dm_participants p WHERE p.room_id = r.id AND p.user_id = $4
)
FROM dm_invites i
JOIN dm_rooms r ON r.id = i.room_id
JOIN dm_room_keys k ON k.room_id = r.id
WHERE i.id = $1
  AND i.used_at IS NULL
  AND i.expires_at > now()
  AND r.company_id = $2
  AND r.domain_id = $3`
	var roomID string
	var key []byte
	var alreadyParticipant bool
	if err := s.db.QueryRowContext(ctx, query, token, principal.CompanyID, principal.DomainID, principal.UserID).Scan(&roomID, &key, &alreadyParticipant); err != nil {
		return "", nil, mapNoRows(err)
	}
	if alreadyParticipant {
		return "", nil, fmt.Errorf("%w: user is already a participant", ErrConflict)
	}
	return roomID, key, nil
}

func (s *PostgresStore) JoinInvite(ctx context.Context, principal Principal, token string, systemMessage MessageRecord) (MessageRecord, error) {
	if err := s.requireDB(); err != nil {
		return MessageRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MessageRecord{}, err
	}
	defer tx.Rollback()
	invite, err := lockInviteTx(ctx, tx, principal, token)
	if err != nil {
		return MessageRecord{}, err
	}
	if err := ensureUsersInScope(ctx, tx, principal, []string{principal.UserID}); err != nil {
		return MessageRecord{}, err
	}
	if existing, err := countParticipantsTx(ctx, tx, invite.RoomID, []string{principal.UserID}); err != nil {
		return MessageRecord{}, err
	} else if existing > 0 {
		return MessageRecord{}, fmt.Errorf("%w: user is already a participant", ErrConflict)
	}
	if err := insertParticipantsTx(ctx, tx, invite.RoomID, []string{principal.UserID}); err != nil {
		return MessageRecord{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dm_invites SET used_at = now(), used_by = $2 WHERE id = $1`, token, principal.UserID); err != nil {
		return MessageRecord{}, err
	}
	systemMessage.RoomID = invite.RoomID
	systemMessage.MessageType = MessageTypeSystem
	systemMessage.SenderID = ""
	record, err := insertMessageTx(ctx, tx, systemMessage)
	if err != nil {
		return MessageRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return MessageRecord{}, err
	}
	return record, nil
}

func (s *PostgresStore) RoomKeyForParticipant(ctx context.Context, principal Principal, roomID string) ([]byte, error) {
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	const query = `
SELECT k.key_ciphertext
FROM dm_room_keys k
JOIN dm_rooms r ON r.id = k.room_id
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $1
WHERE r.id = $2 AND r.company_id = $3 AND r.domain_id = $4`
	var key []byte
	if err := s.db.QueryRowContext(ctx, query, principal.UserID, roomID, principal.CompanyID, principal.DomainID).Scan(&key); err != nil {
		return nil, mapNoRows(err)
	}
	return key, nil
}

func (s *PostgresStore) RoomKeyForMessageOwner(ctx context.Context, principal Principal, messageID string) (string, []byte, error) {
	if err := s.requireDB(); err != nil {
		return "", nil, err
	}
	const query = `
SELECT m.room_id::text, k.key_ciphertext
FROM dm_messages m
JOIN dm_rooms r ON r.id = m.room_id
JOIN dm_room_keys k ON k.room_id = r.id
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $1
WHERE m.id = $2
  AND m.sender_id = $1
  AND m.message_type = 'text'
  AND m.deleted_at IS NULL
  AND r.company_id = $3
  AND r.domain_id = $4`
	var roomID string
	var key []byte
	if err := s.db.QueryRowContext(ctx, query, principal.UserID, messageID, principal.CompanyID, principal.DomainID).Scan(&roomID, &key); err != nil {
		return "", nil, mapNoRows(err)
	}
	return roomID, key, nil
}

func (s *PostgresStore) AttachmentByMessageID(ctx context.Context, messageID string) (MessageRecord, []byte, error) {
	if err := s.requireDB(); err != nil {
		return MessageRecord{}, nil, err
	}
	const query = `
SELECT m.id::text, m.room_id::text, COALESCE(m.sender_id::text, ''), m.message_type, m.body,
  COALESCE(m.attachment_storage_path, NULL), COALESCE(m.attachment_name, ''), COALESCE(m.attachment_size, 0),
  COALESCE(m.attachment_mime_type, ''), COALESCE(m.drive_file_id::text, ''),
  m.created_at, m.edited_at, m.deleted_at, 0, '[]',
  k.key_ciphertext
FROM dm_messages m
JOIN dm_rooms r ON r.id = m.room_id
JOIN dm_room_keys k ON k.room_id = r.id
WHERE m.id = $1 AND m.message_type = 'file' AND m.deleted_at IS NULL`
	var wrapped []byte
	record, err := scanMessageRecordWithTail(s.db.QueryRowContext(ctx, query, messageID), &wrapped)
	if err != nil {
		return MessageRecord{}, nil, mapNoRows(err)
	}
	if len(record.AttachmentStoragePathCiphertext) == 0 {
		return MessageRecord{}, nil, ErrNotFound
	}
	return record, append([]byte(nil), wrapped...), nil
}

// RotateRoomKey replaces the room's encryption key and atomically updates every
// message ciphertext in a single transaction. Only a current participant of the
// room may call this; the SELECT in the query enforces that constraint.
func (s *PostgresStore) RotateRoomKey(ctx context.Context, principal Principal, roomID string, newKeyCiphertext []byte, updatedMessages []MessageRecord) error {
	if err := s.requireDB(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Verify participant and update the room key atomically.
	const updateKey = `
UPDATE dm_room_keys k
SET key_ciphertext = $1
FROM dm_rooms r
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $2
WHERE k.room_id = r.id AND r.id = $3 AND r.company_id = $4 AND r.domain_id = $5`
	res, err := tx.ExecContext(ctx, updateKey, newKeyCiphertext, principal.UserID, roomID, principal.CompanyID, principal.DomainID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}

	// Re-encrypt each message's body and attachment path in the same transaction.
	const updateMsg = `
UPDATE dm_messages SET body = $1, attachment_storage_path = $2 WHERE id = $3::uuid`
	for _, r := range updatedMessages {
		if _, err := tx.ExecContext(ctx, updateMsg, r.BodyCiphertext, nullBytes(r.AttachmentStoragePathCiphertext), r.ID); err != nil {
			return fmt.Errorf("update message %s: %w", r.ID, err)
		}
	}

	return tx.Commit()
}

func lockInviteTx(ctx context.Context, tx *sql.Tx, principal Principal, token string) (Invite, error) {
	const query = `
SELECT i.id::text, i.room_id::text, i.expires_at
FROM dm_invites i
JOIN dm_rooms r ON r.id = i.room_id
WHERE i.id = $1
  AND i.used_at IS NULL
  AND i.expires_at > now()
  AND r.company_id = $2
  AND r.domain_id = $3
  AND r.room_type = 'group'
  AND r.visibility = 'private'
FOR UPDATE OF i`
	var invite Invite
	if err := tx.QueryRowContext(ctx, query, token, principal.CompanyID, principal.DomainID).Scan(&invite.Token, &invite.RoomID, &invite.ExpiresAt); err != nil {
		return Invite{}, mapNoRows(err)
	}
	return invite, nil
}

func insertRoomKeyTx(ctx context.Context, tx *sql.Tx, roomID string, keyCiphertext []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO dm_room_keys (room_id, key_ciphertext) VALUES ($1, $2)`, roomID, keyCiphertext)
	return err
}

func validateUUIDs(ids []string) error {
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return fmt.Errorf("%w: user_ids must be UUIDs", ErrInvalid)
		}
	}
	return nil
}
