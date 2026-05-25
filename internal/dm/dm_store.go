package dm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) CreateDirectRoom(ctx context.Context, principal Principal, otherUserID string, keyCiphertext []byte) (Room, error) {
	if err := s.requireDB(); err != nil {
		return Room{}, err
	}
	otherUserID = strings.TrimSpace(otherUserID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Room{}, err
	}
	defer tx.Rollback()
	if err := ensureUsersInScope(ctx, tx, principal, []string{principal.UserID, otherUserID}); err != nil {
		return Room{}, err
	}
	pair := sortedPair(principal.UserID, otherUserID)
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, principal.DomainID+":"+pair[0]+":"+pair[1]); err != nil {
		return Room{}, fmt.Errorf("lock dm direct room: %w", err)
	}
	if room, ok, err := findDirectRoomTx(ctx, tx, principal, pair[0], pair[1]); err != nil {
		return Room{}, err
	} else if ok {
		room.CurrentUserID = principal.UserID
		if rooms, err := loadMembersForRooms(ctx, tx, []Room{room}); err != nil {
			return Room{}, err
		} else if len(rooms) == 1 {
			room = rooms[0]
		}
		if err := tx.Commit(); err != nil {
			return Room{}, err
		}
		return room, nil
	}
	room, err := insertRoomTx(ctx, tx, principal, RoomTypeDirect, "", "", "")
	if err != nil {
		return Room{}, err
	}
	if err := insertRoomKeyTx(ctx, tx, room.ID, keyCiphertext); err != nil {
		return Room{}, err
	}
	if err := insertParticipantsTx(ctx, tx, room.ID, []string{principal.UserID, otherUserID}); err != nil {
		return Room{}, err
	}
	room.Members, _ = usersInScope(ctx, tx, principal, []string{principal.UserID, otherUserID})
	room.CurrentUserID = principal.UserID
	if err := tx.Commit(); err != nil {
		return Room{}, err
	}
	return room, nil
}

func (s *PostgresStore) CreateGroupRoom(ctx context.Context, principal Principal, req CreateRoomRequest, keyCiphertext []byte) (Room, error) {
	if err := s.requireDB(); err != nil {
		return Room{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Room{}, err
	}
	defer tx.Rollback()
	memberIDs := append([]string{principal.UserID}, req.UserIDs...)
	memberIDs = cleanIDs(memberIDs)
	if err := ensureUsersInScope(ctx, tx, principal, memberIDs); err != nil {
		return Room{}, err
	}
	room, err := insertRoomTx(ctx, tx, principal, RoomTypeGroup, req.Visibility, req.Name, principal.UserID)
	if err != nil {
		return Room{}, err
	}
	if err := insertRoomKeyTx(ctx, tx, room.ID, keyCiphertext); err != nil {
		return Room{}, err
	}
	if err := insertParticipantsTx(ctx, tx, room.ID, memberIDs); err != nil {
		return Room{}, err
	}
	room.Members, _ = usersInScope(ctx, tx, principal, memberIDs)
	room.CurrentUserID = principal.UserID
	if err := tx.Commit(); err != nil {
		return Room{}, err
	}
	return room, nil
}

func (s *PostgresStore) ListRooms(ctx context.Context, principal Principal) ([]Room, error) {
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	const query = `
SELECT
  r.id::text, r.company_id::text, r.domain_id::text, r.room_type, COALESCE(r.visibility, ''),
  COALESCE(r.name, ''), COALESCE(r.owner_id::text, ''), r.created_by::text, r.created_at,
  COALESCE((
    SELECT COUNT(*) FROM dm_messages m
    WHERE m.room_id = r.id
      AND m.sender_id IS DISTINCT FROM $1::uuid
      AND m.deleted_at IS NULL
      AND (
        p.last_read_message_id IS NULL OR
        m.created_at > COALESCE((SELECT created_at FROM dm_messages lr WHERE lr.id = p.last_read_message_id), '-infinity'::timestamptz)
      )
  ), 0)::int AS unread_count,
  COALESCE((SELECT COUNT(*) FROM dm_participants mp WHERE mp.room_id = r.id), 0)::int AS member_count,
  COALESCE(p.last_read_message_id::text, '')
FROM dm_rooms r
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $1
WHERE r.company_id = $2 AND r.domain_id = $3
ORDER BY COALESCE((SELECT MAX(created_at) FROM dm_messages lm WHERE lm.room_id = r.id), r.created_at) DESC, r.created_at DESC`
	rows, err := s.db.QueryContext(ctx, query, principal.UserID, principal.CompanyID, principal.DomainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rooms []Room
	for rows.Next() {
		room, err := scanRoom(rows)
		if err != nil {
			return nil, err
		}
		room.CurrentUserID = principal.UserID
		rooms = append(rooms, room)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return loadMembersForRooms(ctx, s.db, rooms)
}

func (s *PostgresStore) ListPublicRooms(ctx context.Context, principal Principal) ([]Room, error) {
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	const query = `
SELECT
  r.id::text, r.company_id::text, r.domain_id::text, r.room_type, COALESCE(r.visibility, ''),
  COALESCE(r.name, ''), COALESCE(r.owner_id::text, ''), r.created_by::text, r.created_at,
  0, COALESCE((SELECT COUNT(*) FROM dm_participants mp WHERE mp.room_id = r.id), 0)::int, ''
FROM dm_rooms r
WHERE r.company_id = $1 AND r.domain_id = $2 AND r.room_type = 'group' AND r.visibility = 'public'
ORDER BY r.created_at DESC`
	rows, err := s.db.QueryContext(ctx, query, principal.CompanyID, principal.DomainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rooms []Room
	for rows.Next() {
		room, err := scanRoom(rows)
		if err != nil {
			return nil, err
		}
		room.CurrentUserID = principal.UserID
		rooms = append(rooms, room)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return loadMembersForRooms(ctx, s.db, rooms)
}

func (s *PostgresStore) Users(ctx context.Context, principal Principal, userIDs []string) ([]User, error) {
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	return usersInScopeDB(ctx, s.db, principal, userIDs)
}

func (s *PostgresStore) AddMembers(ctx context.Context, principal Principal, roomID string, userIDs []string, systemMessages []MessageRecord) ([]MessageRecord, error) {
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	userIDs = cleanIDs(userIDs)
	if len(userIDs) == 0 || len(systemMessages) != len(userIDs) {
		return nil, fmt.Errorf("%w: member system messages are required", ErrInvalid)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	room, err := lockRoomTx(ctx, tx, principal, roomID)
	if err != nil {
		return nil, err
	}
	if room.RoomType != RoomTypeGroup {
		return nil, fmt.Errorf("%w: direct room members cannot be changed", ErrForbidden)
	}
	if room.OwnerID != principal.UserID {
		return nil, fmt.Errorf("%w: only the room owner can add members", ErrForbidden)
	}
	if err := ensureUsersInScope(ctx, tx, principal, userIDs); err != nil {
		return nil, err
	}
	existing, err := countParticipantsTx(ctx, tx, roomID, userIDs)
	if err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, fmt.Errorf("%w: user is already a participant", ErrConflict)
	}
	if err := insertParticipantsTx(ctx, tx, roomID, userIDs); err != nil {
		return nil, err
	}
	records := make([]MessageRecord, 0, len(systemMessages))
	for _, msg := range systemMessages {
		msg.RoomID = roomID
		msg.MessageType = MessageTypeSystem
		msg.SenderID = ""
		record, err := insertMessageTx(ctx, tx, msg)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *PostgresStore) RemoveMember(ctx context.Context, principal Principal, roomID string, targetUserID string, systemMessage MessageRecord) (RoomRemoval, error) {
	if err := s.requireDB(); err != nil {
		return RoomRemoval{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RoomRemoval{}, err
	}
	defer tx.Rollback()
	room, err := lockRoomTx(ctx, tx, principal, roomID)
	if err != nil {
		return RoomRemoval{}, err
	}
	targetUserID = strings.TrimSpace(targetUserID)
	if targetUserID == "" {
		return RoomRemoval{}, fmt.Errorf("%w: user_id is required", ErrInvalid)
	}
	if room.RoomType == RoomTypeGroup && room.OwnerID == targetUserID {
		return RoomRemoval{}, fmt.Errorf("%w: owner must transfer ownership before leaving", ErrForbidden)
	}
	if targetUserID != principal.UserID && (room.RoomType != RoomTypeGroup || room.OwnerID != principal.UserID) {
		return RoomRemoval{}, fmt.Errorf("%w: only the room owner can remove another member", ErrForbidden)
	}
	if err := ensureParticipantUserTx(ctx, tx, roomID, targetUserID); err != nil {
		return RoomRemoval{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dm_participants WHERE room_id = $1 AND user_id = $2`, roomID, targetUserID); err != nil {
		return RoomRemoval{}, err
	}
	remaining, err := countRoomParticipantsTx(ctx, tx, roomID)
	if err != nil {
		return RoomRemoval{}, err
	}
	if remaining == 0 {
		if err := hardDeleteRoomTx(ctx, tx, roomID); err != nil {
			return RoomRemoval{}, err
		}
		if err := tx.Commit(); err != nil {
			return RoomRemoval{}, err
		}
		return RoomRemoval{DeletedRoom: true}, nil
	}
	systemMessage.RoomID = roomID
	systemMessage.MessageType = MessageTypeSystem
	systemMessage.SenderID = ""
	record, err := insertMessageTx(ctx, tx, systemMessage)
	if err != nil {
		return RoomRemoval{}, err
	}
	if err := tx.Commit(); err != nil {
		return RoomRemoval{}, err
	}
	return RoomRemoval{SystemMessage: record.Message, systemMessageRecord: record}, nil
}

func (s *PostgresStore) TransferOwner(ctx context.Context, principal Principal, roomID string, targetUserID string, systemMessage MessageRecord) (MessageRecord, error) {
	if err := s.requireDB(); err != nil {
		return MessageRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MessageRecord{}, err
	}
	defer tx.Rollback()
	room, err := lockRoomTx(ctx, tx, principal, roomID)
	if err != nil {
		return MessageRecord{}, err
	}
	if room.RoomType != RoomTypeGroup {
		return MessageRecord{}, fmt.Errorf("%w: direct rooms do not have owners", ErrForbidden)
	}
	if room.OwnerID != principal.UserID {
		return MessageRecord{}, fmt.Errorf("%w: only the room owner can transfer ownership", ErrForbidden)
	}
	targetUserID = strings.TrimSpace(targetUserID)
	if targetUserID == "" || targetUserID == principal.UserID {
		return MessageRecord{}, fmt.Errorf("%w: target owner must be another participant", ErrInvalid)
	}
	if err := ensureParticipantUserTx(ctx, tx, roomID, targetUserID); err != nil {
		return MessageRecord{}, err
	}
	systemMessage.RoomID = roomID
	systemMessage.MessageType = MessageTypeSystem
	systemMessage.SenderID = ""
	record, err := insertMessageTx(ctx, tx, systemMessage)
	if err != nil {
		return MessageRecord{}, err
	}
	res, err := tx.ExecContext(ctx, `UPDATE dm_rooms SET owner_id = $1 WHERE id = $2`, targetUserID, roomID)
	if err != nil {
		return MessageRecord{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return MessageRecord{}, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return MessageRecord{}, err
	}
	return record, nil
}

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

func (s *PostgresStore) InsertMessage(ctx context.Context, principal Principal, msg MessageRecord, urls []string) (MessageRecord, error) {
	if err := s.requireDB(); err != nil {
		return MessageRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MessageRecord{}, err
	}
	defer tx.Rollback()
	if err := ensureParticipantTx(ctx, tx, principal, msg.RoomID); err != nil {
		return MessageRecord{}, err
	}
	const insert = `
INSERT INTO dm_messages (
  room_id, sender_id, message_type, body, attachment_storage_path, attachment_name,
  attachment_size, attachment_mime_type, drive_file_id
) VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, NULLIF($6, ''), $7, NULLIF($8, ''), NULLIF($9, '')::uuid)
RETURNING id::text, created_at`
	var attachmentSize sql.NullInt64
	if msg.AttachmentSize > 0 {
		attachmentSize = sql.NullInt64{Int64: msg.AttachmentSize, Valid: true}
	}
	if err := tx.QueryRowContext(ctx, insert,
		msg.RoomID, msg.SenderID, msg.MessageType, msg.BodyCiphertext, nullBytes(msg.AttachmentStoragePathCiphertext),
		msg.AttachmentName, attachmentSize, msg.AttachmentMIMEType, msg.DriveFileID,
	).Scan(&msg.ID, &msg.CreatedAt); err != nil {
		return MessageRecord{}, err
	}
	if err := insertMessageURLsTx(ctx, tx, msg.ID, msg.RoomID, urls); err != nil {
		return MessageRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return MessageRecord{}, err
	}
	return msg, nil
}

func (s *PostgresStore) ListMessages(ctx context.Context, principal Principal, roomID string, cursor MessageCursor) ([]MessageRecord, error) {
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	if cursor.Limit <= 0 {
		cursor.Limit = 50
	}
	where := `
WHERE r.id = $1 AND r.company_id = $2 AND r.domain_id = $3 AND p.user_id = $4`
	args := []any{roomID, principal.CompanyID, principal.DomainID, principal.UserID, cursor.Limit}
	order := "ORDER BY m.created_at DESC, m.id DESC"
	if strings.TrimSpace(cursor.AfterID) != "" {
		where += ` AND m.created_at > COALESCE((SELECT created_at FROM dm_messages WHERE id = $6), 'infinity'::timestamptz)`
		args = append(args, cursor.AfterID)
		order = "ORDER BY m.created_at ASC, m.id ASC"
	} else if strings.TrimSpace(cursor.BeforeID) != "" {
		where += ` AND m.created_at < COALESCE((SELECT created_at FROM dm_messages WHERE id = $6), '-infinity'::timestamptz)`
		args = append(args, cursor.BeforeID)
	}
	query := messageSelectSQL + "\n" + where + "\n" + order + "\nLIMIT $5"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records, err := scanMessageRecords(rows)
	if err != nil {
		return nil, err
	}
	if cursor.AfterID == "" {
		reverseMessageRecords(records)
	}
	return records, nil
}

func (s *PostgresStore) UpdateTextMessage(ctx context.Context, principal Principal, messageID string, bodyCiphertext []byte, urls []string) (MessageRecord, error) {
	if err := s.requireDB(); err != nil {
		return MessageRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MessageRecord{}, err
	}
	defer tx.Rollback()
	const update = `
UPDATE dm_messages m
SET body = $5, edited_at = now()
FROM dm_rooms r, dm_participants p
WHERE m.id = $1
  AND r.id = m.room_id
  AND p.room_id = r.id
  AND p.user_id = $2
  AND m.sender_id = $2
  AND m.message_type = 'text'
  AND m.deleted_at IS NULL
  AND r.company_id = $3
  AND r.domain_id = $4
RETURNING m.id::text, m.room_id::text, m.sender_id::text, m.message_type, m.body,
  NULL::bytea, COALESCE(m.attachment_name, ''), COALESCE(m.attachment_size, 0),
  COALESCE(m.attachment_mime_type, ''), COALESCE(m.drive_file_id::text, ''),
  m.created_at, m.edited_at, m.deleted_at,
  COALESCE((
    SELECT COUNT(*)
    FROM dm_participants rp
    JOIN dm_messages lr ON lr.id = rp.last_read_message_id
    WHERE rp.room_id = m.room_id AND rp.user_id <> $2::uuid AND lr.created_at >= m.created_at
  ), 0)::int,
  COALESCE((
    SELECT jsonb_agg(jsonb_build_object('emoji', rr.emoji, 'count', rr.count, 'mine', rr.mine) ORDER BY rr.emoji)
    FROM (
      SELECT emoji, COUNT(*)::int AS count, BOOL_OR(user_id = $2::uuid) AS mine
      FROM dm_reactions WHERE message_id = m.id GROUP BY emoji
    ) rr
  ), '[]'::jsonb)::text`
	row := tx.QueryRowContext(ctx, update, messageID, principal.UserID, principal.CompanyID, principal.DomainID, bodyCiphertext)
	record, err := scanMessageRecord(row)
	if err != nil {
		return MessageRecord{}, mapNoRows(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dm_message_urls WHERE message_id = $1`, messageID); err != nil {
		return MessageRecord{}, err
	}
	if err := insertMessageURLsTx(ctx, tx, messageID, record.RoomID, urls); err != nil {
		return MessageRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return MessageRecord{}, err
	}
	return record, nil
}

func (s *PostgresStore) SoftDeleteMessage(ctx context.Context, principal Principal, messageID string) (MessageRecord, error) {
	if err := s.requireDB(); err != nil {
		return MessageRecord{}, err
	}
	const query = `
UPDATE dm_messages m
SET deleted_at = COALESCE(m.deleted_at, now())
FROM dm_rooms r, dm_participants p
WHERE m.id = $1
  AND r.id = m.room_id
  AND p.room_id = r.id
  AND p.user_id = $2
  AND m.sender_id = $2
  AND m.message_type <> 'system'
  AND r.company_id = $3
  AND r.domain_id = $4
RETURNING m.id::text, m.room_id::text, m.sender_id::text, m.message_type, m.body,
  NULL::bytea, COALESCE(m.attachment_name, ''), COALESCE(m.attachment_size, 0),
  COALESCE(m.attachment_mime_type, ''), COALESCE(m.drive_file_id::text, ''),
  m.created_at, m.edited_at, m.deleted_at,
  COALESCE((
    SELECT COUNT(*)
    FROM dm_participants rp
    JOIN dm_messages lr ON lr.id = rp.last_read_message_id
    WHERE rp.room_id = m.room_id AND rp.user_id <> $2::uuid AND lr.created_at >= m.created_at
  ), 0)::int,
  COALESCE((
    SELECT jsonb_agg(jsonb_build_object('emoji', rr.emoji, 'count', rr.count, 'mine', rr.mine) ORDER BY rr.emoji)
    FROM (
      SELECT emoji, COUNT(*)::int AS count, BOOL_OR(user_id = $2::uuid) AS mine
      FROM dm_reactions WHERE message_id = m.id GROUP BY emoji
    ) rr
  ), '[]'::jsonb)::text`
	record, err := scanMessageRecord(s.db.QueryRowContext(ctx, query, messageID, principal.UserID, principal.CompanyID, principal.DomainID))
	if err != nil {
		return MessageRecord{}, mapNoRows(err)
	}
	return record, nil
}

func (s *PostgresStore) ToggleReaction(ctx context.Context, principal Principal, messageID string, emoji string) error {
	if err := s.requireDB(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	const eligible = `
SELECT 1
FROM dm_messages m
JOIN dm_rooms r ON r.id = m.room_id
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $2
WHERE m.id = $1 AND m.message_type <> 'system' AND m.deleted_at IS NULL
  AND r.company_id = $3 AND r.domain_id = $4`
	var one int
	if err := tx.QueryRowContext(ctx, eligible, messageID, principal.UserID, principal.CompanyID, principal.DomainID).Scan(&one); err != nil {
		return mapNoRows(err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM dm_reactions WHERE message_id = $1 AND user_id = $2 AND emoji = $3`, messageID, principal.UserID, emoji)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO dm_reactions (message_id, user_id, emoji) VALUES ($1, $2, $3)`, messageID, principal.UserID, emoji); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresStore) MarkRead(ctx context.Context, principal Principal, roomID string, lastMessageID string) error {
	if err := s.requireDB(); err != nil {
		return err
	}
	const query = `
UPDATE dm_participants p
SET last_read_message_id = $4
FROM dm_rooms r, dm_messages m
WHERE p.room_id = r.id
  AND p.user_id = $1
  AND r.id = $2
  AND r.company_id = $3
  AND r.domain_id = $5
  AND m.id = $4
  AND m.room_id = r.id`
	res, err := s.db.ExecContext(ctx, query, principal.UserID, roomID, principal.CompanyID, lastMessageID, principal.DomainID)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) ListSearchCandidates(ctx context.Context, principal Principal, roomID string, beforeMessageID string, limit int) ([]MessageRecord, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	cursor := MessageCursor{BeforeID: beforeMessageID, Limit: limit}
	records, err := s.ListMessages(ctx, principal, roomID, cursor)
	if err != nil {
		return nil, err
	}
	reverseMessageRecords(records)
	return records, nil
}

func (s *PostgresStore) ListMedia(ctx context.Context, principal Principal, roomID string, query MediaQuery) ([]MediaItem, error) {
	if err := s.requireDB(); err != nil {
		return nil, err
	}
	mediaType := strings.ToLower(strings.TrimSpace(query.Type))
	if mediaType == "links" {
		return s.listLinkMedia(ctx, principal, roomID, query)
	}
	if mediaType == "drive" {
		return s.listDriveMedia(ctx, principal, roomID, query)
	}
	return s.listFileMedia(ctx, principal, roomID, query)
}

func (s *PostgresStore) requireDB() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("dm database handle is required")
	}
	return nil
}

const messageSelectSQL = `
SELECT m.id::text, m.room_id::text, COALESCE(m.sender_id::text, ''), m.message_type, m.body,
  COALESCE(m.attachment_storage_path, NULL), COALESCE(m.attachment_name, ''), COALESCE(m.attachment_size, 0),
  COALESCE(m.attachment_mime_type, ''), COALESCE(m.drive_file_id::text, ''),
  m.created_at, m.edited_at, m.deleted_at,
  COALESCE((
    SELECT COUNT(*)
    FROM dm_participants rp
    JOIN dm_messages lr ON lr.id = rp.last_read_message_id
    WHERE rp.room_id = m.room_id AND rp.user_id <> $4::uuid AND lr.created_at >= m.created_at
  ), 0)::int AS read_count,
  COALESCE((
    SELECT jsonb_agg(jsonb_build_object('emoji', rr.emoji, 'count', rr.count, 'mine', rr.mine) ORDER BY rr.emoji)
    FROM (
      SELECT emoji, COUNT(*)::int AS count, BOOL_OR(user_id = $4::uuid) AS mine
      FROM dm_reactions WHERE message_id = m.id GROUP BY emoji
    ) rr
  ), '[]'::jsonb)::text AS reactions
FROM dm_messages m
JOIN dm_rooms r ON r.id = m.room_id
JOIN dm_participants p ON p.room_id = r.id`

type scanner interface {
	Scan(dest ...any) error
}

type roomMemberQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func scanRoom(row scanner) (Room, error) {
	var room Room
	err := row.Scan(&room.ID, &room.CompanyID, &room.DomainID, &room.RoomType, &room.Visibility, &room.Name, &room.OwnerID, &room.CreatedBy, &room.CreatedAt, &room.UnreadCount, &room.MemberCount, &room.LastReadID)
	return room, err
}

func loadMembersForRooms(ctx context.Context, q roomMemberQuerier, rooms []Room) ([]Room, error) {
	if len(rooms) == 0 {
		return rooms, nil
	}
	roomIDs := make([]string, 0, len(rooms))
	for _, room := range rooms {
		roomIDs = append(roomIDs, room.ID)
	}
	const query = `
SELECT p.room_id::text, u.id::text, d.company_id::text, u.domain_id::text, u.display_name, COALESCE(primary_addr.address, ''), COALESCE(u.settings->>'avatar_url', '')
FROM dm_participants p
JOIN users u ON u.id = p.user_id
JOIN domains d ON d.id = u.domain_id
LEFT JOIN LATERAL (
  SELECT ua.address
  FROM user_addresses ua
  WHERE ua.user_id = u.id
  ORDER BY ua.is_primary DESC, ua.address ASC
  LIMIT 1
) primary_addr ON true
WHERE p.room_id = ANY($1::uuid[])
ORDER BY p.joined_at ASC, u.display_name ASC`
	rows, err := q.QueryContext(ctx, query, pq.Array(roomIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	membersByRoom := make(map[string][]User)
	for rows.Next() {
		var roomID string
		var u User
		if err := rows.Scan(&roomID, &u.ID, &u.CompanyID, &u.DomainID, &u.DisplayName, &u.Email, &u.AvatarURL); err != nil {
			return nil, err
		}
		membersByRoom[roomID] = append(membersByRoom[roomID], u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range rooms {
		rooms[i].Members = membersByRoom[rooms[i].ID]
		if rooms[i].MemberCount == 0 {
			rooms[i].MemberCount = len(rooms[i].Members)
		}
	}
	return rooms, nil
}

func scanMessageRecord(row scanner) (MessageRecord, error) {
	return scanMessageRecordWithTail(row)
}

func scanMessageRecordWithTail(row scanner, tail ...any) (MessageRecord, error) {
	var record MessageRecord
	var editedAt, deletedAt sql.NullTime
	var attachmentPath []byte
	var reactionsJSON string
	dest := []any{
		&record.ID, &record.RoomID, &record.SenderID, &record.MessageType, &record.BodyCiphertext,
		&attachmentPath, &record.AttachmentName, &record.AttachmentSize, &record.AttachmentMIMEType,
		&record.DriveFileID, &record.CreatedAt, &editedAt, &deletedAt, &record.ReadCount, &reactionsJSON,
	}
	dest = append(dest, tail...)
	if err := row.Scan(dest...); err != nil {
		return MessageRecord{}, err
	}
	record.AttachmentStoragePathCiphertext = attachmentPath
	if editedAt.Valid {
		record.EditedAt = &editedAt.Time
	}
	if deletedAt.Valid {
		record.DeletedAt = &deletedAt.Time
	}
	if reactionsJSON != "" && reactionsJSON != "[]" {
		if err := json.Unmarshal([]byte(reactionsJSON), &record.Reactions); err != nil {
			return MessageRecord{}, err
		}
	}
	return record, nil
}

func scanMessageRecords(rows *sql.Rows) ([]MessageRecord, error) {
	var records []MessageRecord
	for rows.Next() {
		record, err := scanMessageRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func insertRoomTx(ctx context.Context, tx *sql.Tx, principal Principal, roomType, visibility, name, ownerID string) (Room, error) {
	const query = `
INSERT INTO dm_rooms (company_id, domain_id, room_type, visibility, name, owner_id, created_by)
VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, '')::uuid, $7)
RETURNING id::text, company_id::text, domain_id::text, room_type, COALESCE(visibility, ''),
  COALESCE(name, ''), COALESCE(owner_id::text, ''), created_by::text, created_at, 0, 0, ''`
	return scanRoom(tx.QueryRowContext(ctx, query, principal.CompanyID, principal.DomainID, roomType, visibility, name, ownerID, principal.UserID))
}

func insertRoomKeyTx(ctx context.Context, tx *sql.Tx, roomID string, keyCiphertext []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO dm_room_keys (room_id, key_ciphertext) VALUES ($1, $2)`, roomID, keyCiphertext)
	return err
}

func insertParticipantsTx(ctx context.Context, tx *sql.Tx, roomID string, userIDs []string) error {
	for _, id := range cleanIDs(userIDs) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO dm_participants (room_id, user_id) VALUES ($1, $2) ON CONFLICT (room_id, user_id) DO NOTHING`, roomID, id); err != nil {
			return err
		}
	}
	return nil
}

func insertMessageTx(ctx context.Context, tx *sql.Tx, msg MessageRecord) (MessageRecord, error) {
	const insert = `
INSERT INTO dm_messages (
  room_id, sender_id, message_type, body, attachment_storage_path, attachment_name,
  attachment_size, attachment_mime_type, drive_file_id
) VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, NULLIF($6, ''), $7, NULLIF($8, ''), NULLIF($9, '')::uuid)
RETURNING id::text, created_at`
	var attachmentSize sql.NullInt64
	if msg.AttachmentSize > 0 {
		attachmentSize = sql.NullInt64{Int64: msg.AttachmentSize, Valid: true}
	}
	if err := tx.QueryRowContext(ctx, insert,
		msg.RoomID, msg.SenderID, msg.MessageType, msg.BodyCiphertext, nullBytes(msg.AttachmentStoragePathCiphertext),
		msg.AttachmentName, attachmentSize, msg.AttachmentMIMEType, msg.DriveFileID,
	).Scan(&msg.ID, &msg.CreatedAt); err != nil {
		return MessageRecord{}, err
	}
	return msg, nil
}

const findDirectRoomQuery = `
SELECT r.id::text, r.company_id::text, r.domain_id::text, r.room_type, COALESCE(r.visibility, ''),
  COALESCE(r.name, ''), COALESCE(r.owner_id::text, ''), r.created_by::text, r.created_at,
  0, 2, ''
FROM dm_rooms r
JOIN dm_participants p1 ON p1.room_id = r.id AND p1.user_id = $3
JOIN dm_participants p2 ON p2.room_id = r.id AND p2.user_id = $4
WHERE r.company_id = $1 AND r.domain_id = $2 AND r.room_type = 'direct'
  AND (SELECT COUNT(*) FROM dm_participants pc WHERE pc.room_id = r.id) = 2
LIMIT 1`

func findDirectRoomTx(ctx context.Context, tx *sql.Tx, principal Principal, a string, b string) (Room, bool, error) {
	room, err := scanRoom(tx.QueryRowContext(ctx, findDirectRoomQuery, principal.CompanyID, principal.DomainID, a, b))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Room{}, false, nil
		}
		return Room{}, false, err
	}
	return room, true, nil
}

func lockRoomTx(ctx context.Context, tx *sql.Tx, principal Principal, roomID string) (Room, error) {
	const query = `
SELECT r.id::text, r.company_id::text, r.domain_id::text, r.room_type, COALESCE(r.visibility, ''),
  COALESCE(r.name, ''), COALESCE(r.owner_id::text, ''), r.created_by::text, r.created_at,
  0, COALESCE((SELECT COUNT(*) FROM dm_participants mp WHERE mp.room_id = r.id), 0)::int, ''
FROM dm_rooms r
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $1
WHERE r.id = $2 AND r.company_id = $3 AND r.domain_id = $4
FOR UPDATE OF r`
	room, err := scanRoom(tx.QueryRowContext(ctx, query, principal.UserID, roomID, principal.CompanyID, principal.DomainID))
	if err != nil {
		return Room{}, mapNoRows(err)
	}
	return room, nil
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

func ensureUsersInScope(ctx context.Context, tx *sql.Tx, principal Principal, userIDs []string) error {
	users, err := usersInScope(ctx, tx, principal, userIDs)
	if err != nil {
		return err
	}
	if len(users) != len(cleanIDs(userIDs)) {
		return fmt.Errorf("%w: users must belong to the same domain", ErrInvalid)
	}
	return nil
}

func usersInScope(ctx context.Context, tx *sql.Tx, principal Principal, userIDs []string) ([]User, error) {
	ids := cleanIDs(userIDs)
	if len(ids) == 0 {
		return nil, nil
	}
	if err := validateUUIDs(ids); err != nil {
		return nil, err
	}
	const query = `
SELECT u.id::text, d.company_id::text, u.domain_id::text, u.display_name, COALESCE(primary_addr.address, ''), COALESCE(u.settings->>'avatar_url', '')
FROM users u
JOIN domains d ON d.id = u.domain_id
LEFT JOIN LATERAL (
  SELECT ua.address
  FROM user_addresses ua
  WHERE ua.user_id = u.id
  ORDER BY ua.is_primary DESC, ua.address ASC
  LIMIT 1
) primary_addr ON true
WHERE u.id = ANY($1::uuid[])
  AND u.domain_id = $2
  AND d.company_id = $3
  AND u.status = 'active'
  AND d.status = 'active'`
	rows, err := tx.QueryContext(ctx, query, pq.Array(ids), principal.DomainID, principal.CompanyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.CompanyID, &u.DomainID, &u.DisplayName, &u.Email, &u.AvatarURL); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func usersInScopeDB(ctx context.Context, db *sql.DB, principal Principal, userIDs []string) ([]User, error) {
	ids := cleanIDs(userIDs)
	if len(ids) == 0 {
		return nil, nil
	}
	if err := validateUUIDs(ids); err != nil {
		return nil, err
	}
	const query = `
SELECT u.id::text, d.company_id::text, u.domain_id::text, u.display_name, COALESCE(primary_addr.address, ''), COALESCE(u.settings->>'avatar_url', '')
FROM users u
JOIN domains d ON d.id = u.domain_id
LEFT JOIN LATERAL (
  SELECT ua.address
  FROM user_addresses ua
  WHERE ua.user_id = u.id
  ORDER BY ua.is_primary DESC, ua.address ASC
  LIMIT 1
) primary_addr ON true
WHERE u.id = ANY($1::uuid[])
  AND u.domain_id = $2
  AND d.company_id = $3
  AND u.status = 'active'
  AND d.status = 'active'`
	rows, err := db.QueryContext(ctx, query, pq.Array(ids), principal.DomainID, principal.CompanyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.CompanyID, &u.DomainID, &u.DisplayName, &u.Email, &u.AvatarURL); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func ensureParticipantTx(ctx context.Context, tx *sql.Tx, principal Principal, roomID string) error {
	const query = `
SELECT 1
FROM dm_rooms r
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $1
WHERE r.id = $2 AND r.company_id = $3 AND r.domain_id = $4`
	var one int
	if err := tx.QueryRowContext(ctx, query, principal.UserID, roomID, principal.CompanyID, principal.DomainID).Scan(&one); err != nil {
		return mapNoRows(err)
	}
	return nil
}

func ensureParticipantUserTx(ctx context.Context, tx *sql.Tx, roomID string, userID string) error {
	const query = `SELECT 1 FROM dm_participants WHERE room_id = $1 AND user_id = $2`
	var one int
	if err := tx.QueryRowContext(ctx, query, roomID, userID).Scan(&one); err != nil {
		return mapNoRows(err)
	}
	return nil
}

func countParticipantsTx(ctx context.Context, tx *sql.Tx, roomID string, userIDs []string) (int, error) {
	ids := cleanIDs(userIDs)
	if len(ids) == 0 {
		return 0, nil
	}
	const query = `SELECT COUNT(*) FROM dm_participants WHERE room_id = $1 AND user_id = ANY($2::uuid[])`
	var count int
	if err := tx.QueryRowContext(ctx, query, roomID, pq.Array(ids)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func countRoomParticipantsTx(ctx context.Context, tx *sql.Tx, roomID string) (int, error) {
	const query = `SELECT COUNT(*) FROM dm_participants WHERE room_id = $1`
	var count int
	if err := tx.QueryRowContext(ctx, query, roomID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func hardDeleteRoomTx(ctx context.Context, tx *sql.Tx, roomID string) error {
	for _, stmt := range hardDeleteRoomStatements() {
		if _, err := tx.ExecContext(ctx, stmt, roomID); err != nil {
			return err
		}
	}
	return nil
}

func hardDeleteRoomStatements() []string {
	return []string{
		`DELETE FROM dm_room_keys WHERE room_id = $1`,
		`DELETE FROM dm_messages WHERE room_id = $1`,
		`DELETE FROM dm_participants WHERE room_id = $1`,
		`DELETE FROM dm_invites WHERE room_id = $1`,
		`DELETE FROM dm_rooms WHERE id = $1`,
	}
}

func insertMessageURLsTx(ctx context.Context, tx *sql.Tx, messageID string, roomID string, urls []string) error {
	for _, u := range urls {
		if _, err := tx.ExecContext(ctx, `INSERT INTO dm_message_urls (message_id, room_id, url) VALUES ($1, $2, $3)`, messageID, roomID, u); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) listLinkMedia(ctx context.Context, principal Principal, roomID string, query MediaQuery) ([]MediaItem, error) {
	const sqlQuery = `
SELECT u.message_id::text, m.sender_id::text, u.url, u.created_at
FROM dm_message_urls u
JOIN dm_messages m ON m.id = u.message_id
JOIN dm_rooms r ON r.id = u.room_id
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $1
WHERE r.id = $2 AND r.company_id = $3 AND r.domain_id = $4
ORDER BY u.created_at DESC
LIMIT $5`
	rows, err := s.db.QueryContext(ctx, sqlQuery, principal.UserID, roomID, principal.CompanyID, principal.DomainID, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MediaItem
	for rows.Next() {
		var item MediaItem
		item.MessageType = "link"
		if err := rows.Scan(&item.MessageID, &item.SenderID, &item.URL, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) listDriveMedia(ctx context.Context, principal Principal, roomID string, query MediaQuery) ([]MediaItem, error) {
	const sqlQuery = `
SELECT m.id::text, COALESCE(m.sender_id::text, ''), m.drive_file_id::text, COALESCE(n.name, ''), m.created_at
FROM dm_messages m
JOIN dm_rooms r ON r.id = m.room_id
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $1
LEFT JOIN drive_nodes n ON n.id = m.drive_file_id
WHERE r.id = $2 AND r.company_id = $3 AND r.domain_id = $4 AND m.drive_file_id IS NOT NULL AND m.deleted_at IS NULL
ORDER BY m.created_at DESC
LIMIT $5`
	rows, err := s.db.QueryContext(ctx, sqlQuery, principal.UserID, roomID, principal.CompanyID, principal.DomainID, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MediaItem
	for rows.Next() {
		var item MediaItem
		item.MessageType = MessageTypeDriveLink
		if err := rows.Scan(&item.MessageID, &item.SenderID, &item.DriveFileID, &item.DriveName, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) listFileMedia(ctx context.Context, principal Principal, roomID string, query MediaQuery) ([]MediaItem, error) {
	filter := ""
	switch strings.ToLower(strings.TrimSpace(query.Type)) {
	case "image":
		filter = " AND m.attachment_mime_type LIKE 'image/%'"
	case "video":
		filter = " AND m.attachment_mime_type LIKE 'video/%'"
	case "file":
		filter = " AND (m.attachment_mime_type IS NULL OR (m.attachment_mime_type NOT LIKE 'image/%' AND m.attachment_mime_type NOT LIKE 'video/%'))"
	}
	sqlQuery := `
SELECT m.id::text, COALESCE(m.sender_id::text, ''), COALESCE(m.attachment_name, ''), COALESCE(m.attachment_size, 0),
  COALESCE(m.attachment_mime_type, ''), m.created_at
FROM dm_messages m
JOIN dm_rooms r ON r.id = m.room_id
JOIN dm_participants p ON p.room_id = r.id AND p.user_id = $1
WHERE r.id = $2 AND r.company_id = $3 AND r.domain_id = $4 AND m.message_type = 'file' AND m.deleted_at IS NULL` + filter + `
ORDER BY m.created_at DESC
LIMIT $5`
	rows, err := s.db.QueryContext(ctx, sqlQuery, principal.UserID, roomID, principal.CompanyID, principal.DomainID, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MediaItem
	for rows.Next() {
		var item MediaItem
		item.MessageType = MessageTypeFile
		if err := rows.Scan(&item.MessageID, &item.SenderID, &item.AttachmentName, &item.AttachmentSize, &item.AttachmentMIMEType, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func nullBytes(data []byte) any {
	if len(data) == 0 {
		return nil
	}
	return data
}

func mapNoRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func sortedPair(a, b string) []string {
	if strings.Compare(a, b) <= 0 {
		return []string{a, b}
	}
	return []string{b, a}
}

func validateUUIDs(ids []string) error {
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return fmt.Errorf("%w: user_ids must be UUIDs", ErrInvalid)
		}
	}
	return nil
}

func reverseMessageRecords(records []MessageRecord) {
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}
}
