package maildb

import (
	"errors"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/lib/pq"
)

func (r *Repository) ExistingIMAPMessageUIDs(ctx context.Context, userID string, messageIDs []string) ([]IMAPMessageUID, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	messageIDs = normalizeExistingIMAPMessageIDs(messageIDs)
	if len(messageIDs) == 0 {
		return nil, nil
	}
	const query = `
WITH input AS (
  SELECT value::uuid AS message_id, ordinality
  FROM unnest($2::uuid[]) WITH ORDINALITY AS input(value, ordinality)
)
SELECT
  imu.message_id::text,
  imu.mailbox_id::text,
  imu.uid,
  (
    SELECT COUNT(*)
    FROM imap_message_uid seq
    WHERE seq.user_id = imu.user_id
      AND seq.mailbox_id = imu.mailbox_id
      AND seq.uid <= imu.uid
  )::integer AS sequence_number,
  imu.modseq
FROM input
JOIN imap_message_uid imu
  ON imu.message_id = input.message_id
WHERE imu.user_id = $1::uuid
ORDER BY input.ordinality`
	rows, err := r.db.QueryContext(ctx, query, userID, pq.Array(messageIDs))
	if err != nil {
		return nil, fmt.Errorf("list existing imap message uids: %w", err)
	}
	defer rows.Close()

	items := make([]IMAPMessageUID, 0, len(messageIDs))
	for rows.Next() {
		var item IMAPMessageUID
		if err := rows.Scan(&item.MessageID, &item.MailboxID, &item.UID, &item.SequenceNumber, &item.ModSeq); err != nil {
			return nil, fmt.Errorf("scan imap message uid: %w", err)
		}
		if err := imapgw.ValidateMessageUID(item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imap message uids: %w", err)
	}
	return items, nil
}

func normalizeExistingIMAPMessageIDs(messageIDs []string) []string {
	seen := make(map[string]struct{}, len(messageIDs))
	out := make([]string, 0, len(messageIDs))
	for _, id := range messageIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (r *Repository) EnsureIMAPMessageUIDsForMessages(ctx context.Context, userID string, messageIDs []string) ([]IMAPMessageUID, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	messageIDs = normalizeExistingIMAPMessageIDs(messageIDs)
	if len(messageIDs) == 0 {
		return nil, nil
	}
	const query = `
WITH requested AS (
  SELECT value AS message_id, ord
  FROM unnest($2::uuid[]) WITH ORDINALITY AS requested(value, ord)
)
SELECT
  m.id::text,
  m.folder_id::text,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date
FROM requested
JOIN messages m ON m.id = requested.message_id::uuid
WHERE m.user_id = $1::uuid
  AND m.status = 'active'
ORDER BY m.folder_id, internal_date, m.id`
	rows, err := r.db.QueryContext(ctx, query, userID, pq.Array(messageIDs))
	if err != nil {
		return nil, fmt.Errorf("list active imap message uid targets: %w", err)
	}
	type target struct {
		messageID    string
		mailboxID    string
		internalDate time.Time
	}
	targets := make([]target, 0, len(messageIDs))
	for rows.Next() {
		var item target
		if err := rows.Scan(&item.messageID, &item.mailboxID, &item.internalDate); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan active imap message uid target: %w", err)
		}
		targets = append(targets, item)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close active imap message uid targets: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active imap message uid targets: %w", err)
	}

	assigned := make([]IMAPMessageUID, 0, len(targets))
	for _, target := range targets {
		uid, err := r.EnsureIMAPMessageUID(ctx, userID, target.mailboxID, target.messageID)
		if err != nil {
			return nil, err
		}
		sequenceNumber, err := imapSequenceNumberForUID(ctx, r.db, userID, target.mailboxID, uid.UID)
		if err != nil {
			return nil, err
		}
		uid.SequenceNumber = sequenceNumber
		assigned = append(assigned, uid)
	}
	return assigned, nil
}

func (r *Repository) BackfillIMAPMailboxUIDs(ctx context.Context, userID string, mailboxID string, limit int) ([]IMAPMessageUID, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(mailboxID) == "" {
		return nil, fmt.Errorf("mailbox_id is required")
	}
	limit = normalizeIMAPUIDBackfillLimit(limit)

	if _, err := r.EnsureIMAPMailboxState(ctx, userID, mailboxID); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin imap uid backfill transaction: %w", err)
	}
	defer tx.Rollback()

	var uidNext uint32
	var highestModSeq uint64
	const lockMailbox = `
SELECT uidnext, highest_modseq
FROM imap_mailbox_state
WHERE mailbox_id = $1::uuid
  AND user_id = $2::uuid
FOR UPDATE`
	if err := tx.QueryRowContext(ctx, lockMailbox, mailboxID, userID).Scan(&uidNext, &highestModSeq); err != nil {
		return nil, fmt.Errorf("lock imap mailbox state: %w", err)
	}
	if err := lockIMAPMailboxFolderForUIDAllocation(ctx, tx, userID, mailboxID); err != nil {
		return nil, err
	}

	const selectMessages = `
SELECT m.id::text
FROM messages m
LEFT JOIN imap_message_uid i ON i.message_id = m.id
WHERE m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'
  AND i.message_id IS NULL
ORDER BY COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at), m.id
LIMIT $3
FOR UPDATE OF m`
	rows, err := tx.QueryContext(ctx, selectMessages, userID, mailboxID, limit)
	if err != nil {
		return nil, fmt.Errorf("select imap uid backfill messages: %w", err)
	}
	var messageIDs []string
	for rows.Next() {
		var messageID string
		if err := rows.Scan(&messageID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan imap uid backfill message: %w", err)
		}
		messageIDs = append(messageIDs, messageID)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close imap uid backfill rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imap uid backfill messages: %w", err)
	}

	assigned := make([]IMAPMessageUID, 0, len(messageIDs))
	const insertUID = `
INSERT INTO imap_message_uid (message_id, mailbox_id, user_id, uid, modseq)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5)
	RETURNING message_id::text, mailbox_id::text, uid, modseq`
	for _, messageID := range messageIDs {
		if uidNext == 0 || uidNext == math.MaxUint32 {
			return nil, fmt.Errorf("imap uid space exhausted")
		}
		highestModSeq++
		var result IMAPMessageUID
		if err := tx.QueryRowContext(ctx, insertUID, messageID, mailboxID, userID, uidNext, highestModSeq).Scan(
			&result.MessageID,
			&result.MailboxID,
			&result.UID,
			&result.ModSeq,
		); err != nil {
			return nil, fmt.Errorf("insert imap uid backfill row: %w", err)
		}
		if err := imapgw.ValidateMessageUID(result); err != nil {
			return nil, err
		}
		assigned = append(assigned, result)
		uidNext++
	}
	if len(assigned) > 0 {
		const updateMailbox = `
UPDATE imap_mailbox_state
SET uidnext = $2,
    highest_modseq = $3,
    updated_at = now()
WHERE mailbox_id = $1::uuid`
		if _, err := tx.ExecContext(ctx, updateMailbox, mailboxID, uidNext, int64(highestModSeq)); err != nil {
			return nil, fmt.Errorf("update imap uid backfill state: %w", err)
		}
	}
	detail, err := imapUIDBackfillAuditDetail(userID, mailboxID, limit, assigned)
	if err != nil {
		return nil, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		UserID:     userID,
		Category:   "admin",
		Action:     "imap.uid_backfill",
		TargetType: "imap_mailbox",
		TargetID:   mailboxID,
		Result:     "completed",
		Detail:     detail,
	}); err != nil {
		return nil, fmt.Errorf("record imap uid backfill audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit imap uid backfill transaction: %w", err)
	}
	return assigned, nil
}

func imapUIDBackfillAuditDetail(userID string, mailboxID string, limit int, assigned []IMAPMessageUID) (json.RawMessage, error) {
	detail := struct {
		UserID        string                         `json:"user_id"`
		MailboxID     string                         `json:"mailbox_id"`
		Limit         int                            `json:"limit"`
		AssignedCount int                            `json:"assigned_count"`
		Assigned      []imapUIDBackfillAuditSampleID `json:"assigned_sample"`
	}{
		UserID:        strings.TrimSpace(userID),
		MailboxID:     mailboxID,
		Limit:         normalizeIMAPUIDBackfillLimit(limit),
		AssignedCount: len(assigned),
		Assigned:      sampleIMAPUIDBackfillAuditIDs(assigned),
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("marshal imap uid backfill audit detail: %w", err)
	}
	return raw, nil
}

type imapUIDBackfillAuditSampleID struct {
	MessageID string `json:"message_id"`
	UID       uint32 `json:"uid"`
	ModSeq    uint64 `json:"modseq"`
}

func sampleIMAPUIDBackfillAuditIDs(assigned []IMAPMessageUID) []imapUIDBackfillAuditSampleID {
	if len(assigned) > maxIMAPUIDBackfillAuditSample {
		assigned = assigned[:maxIMAPUIDBackfillAuditSample]
	}
	out := make([]imapUIDBackfillAuditSampleID, 0, len(assigned))
	for _, item := range assigned {
		out = append(out, imapUIDBackfillAuditSampleID{
			MessageID: string(item.MessageID),
			UID:       uint32(item.UID),
			ModSeq:    item.ModSeq,
		})
	}
	return out
}

func (r *Repository) EnsureIMAPMessageUID(ctx context.Context, userID string, mailboxID string, messageID string) (IMAPMessageUID, error) {
	if r.db == nil {
		return IMAPMessageUID{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	if userID == "" {
		return IMAPMessageUID{}, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(mailboxID) == "" {
		return IMAPMessageUID{}, fmt.Errorf("mailbox_id is required")
	}
	if messageID == "" {
		return IMAPMessageUID{}, fmt.Errorf("message_id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return IMAPMessageUID{}, fmt.Errorf("begin imap uid transaction: %w", err)
	}
	defer tx.Rollback()

	const ensureState = `
INSERT INTO imap_mailbox_state (mailbox_id, user_id)
SELECT id, user_id
FROM folders
WHERE id = $1::uuid
  AND user_id = $2::uuid
ON CONFLICT (mailbox_id) DO NOTHING`
	if _, err := tx.ExecContext(ctx, ensureState, mailboxID, userID); err != nil {
		return IMAPMessageUID{}, fmt.Errorf("ensure imap mailbox state: %w", err)
	}
	if err := ensureIMAPMessageUIDCapacityTx(ctx, tx, userID, mailboxID, messageID); err != nil {
		return IMAPMessageUID{}, err
	}

	var uid uint32
	var modseq uint64
	const assign = `
WITH mailbox AS (
  SELECT mailbox_id, user_id, uidnext, highest_modseq
  FROM imap_mailbox_state
  WHERE mailbox_id = $2::uuid
    AND user_id = $3::uuid
  FOR UPDATE
),
message AS (
  SELECT m.id, m.folder_id, m.user_id
  FROM messages m
  WHERE m.id = $1::uuid
    AND m.folder_id = $2::uuid
    AND m.user_id = $3::uuid
    AND m.status = 'active'
  FOR UPDATE OF m
),
inserted AS (
  INSERT INTO imap_message_uid (message_id, mailbox_id, user_id, uid, modseq)
  SELECT message.id, mailbox.mailbox_id, mailbox.user_id, mailbox.uidnext, mailbox.highest_modseq + 1
  FROM mailbox, message
  ON CONFLICT (message_id) DO NOTHING
  RETURNING uid, modseq
),
bumped AS (
  UPDATE imap_mailbox_state
  SET uidnext = CASE WHEN EXISTS (SELECT 1 FROM inserted) THEN uidnext + 1 ELSE uidnext END,
      highest_modseq = CASE WHEN EXISTS (SELECT 1 FROM inserted) THEN highest_modseq + 1 ELSE highest_modseq END,
      updated_at = CASE WHEN EXISTS (SELECT 1 FROM inserted) THEN now() ELSE updated_at END
  WHERE mailbox_id = $2::uuid
  RETURNING 1
)
SELECT uid, modseq FROM inserted
UNION ALL
SELECT i.uid, i.modseq
FROM imap_message_uid i
JOIN messages m ON m.id = i.message_id
WHERE i.message_id = $1::uuid
  AND i.mailbox_id = $2::uuid
  AND i.user_id = $3::uuid
  AND m.folder_id = $2::uuid
  AND m.user_id = $3::uuid
  AND m.status = 'active'
LIMIT 1`
	if err := tx.QueryRowContext(ctx, assign, messageID, mailboxID, userID).Scan(&uid, &modseq); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IMAPMessageUID{}, fmt.Errorf("ensure imap message uid for message %q in mailbox %q: %w", messageID, mailboxID, ErrIMAPMessageNotActive)
		}
		return IMAPMessageUID{}, fmt.Errorf("ensure imap message uid: %w", err)
	}

	result := IMAPMessageUID{
		MessageID: imapgw.MessageID(messageID),
		MailboxID: imapgw.MailboxID(mailboxID),
		UID:       imapgw.UID(uid),
		ModSeq:    modseq,
	}
	if err := imapgw.ValidateMessageUID(result); err != nil {
		return IMAPMessageUID{}, err
	}
	if err := tx.Commit(); err != nil {
		return IMAPMessageUID{}, fmt.Errorf("commit imap uid transaction: %w", err)
	}
	return result, nil
}

func ensureIMAPMessageUIDCapacityTx(ctx context.Context, tx *sql.Tx, userID string, mailboxID string, messageID string) error {
	var uidNext imapgw.UID
	const lockState = `
SELECT uidnext
FROM imap_mailbox_state
WHERE mailbox_id = $1::uuid
  AND user_id = $2::uuid
FOR UPDATE`
	if err := tx.QueryRowContext(ctx, lockState, mailboxID, userID).Scan(&uidNext); err != nil {
		return fmt.Errorf("lock imap message uid state: %w", err)
	}
	if err := lockIMAPMailboxFolderForUIDAllocation(ctx, tx, userID, mailboxID); err != nil {
		return err
	}

	var needsUID bool
	const target = `
SELECT EXISTS (
  SELECT 1
  FROM messages m
  LEFT JOIN imap_message_uid i
    ON i.message_id = m.id
   AND i.user_id = m.user_id
   AND i.mailbox_id = m.folder_id
  WHERE m.id = $1::uuid
    AND m.folder_id = $2::uuid
    AND m.user_id = $3::uuid
    AND m.status = 'active'
    AND i.message_id IS NULL
)`
	if err := tx.QueryRowContext(ctx, target, messageID, mailboxID, userID).Scan(&needsUID); err != nil {
		return fmt.Errorf("inspect imap message uid target: %w", err)
	}
	if needsUID && uidNext >= math.MaxUint32 {
		return fmt.Errorf("imap uid space exhausted")
	}
	return nil
}

// LookupIMAPMessageUIDs returns the IMAP UID for each message ID in
// messageIDs that exists in the given mailbox. Messages without an assigned
// UID (not yet processed by the uid-backfill worker) are silently omitted.
func (r *Repository) LookupIMAPMessageUIDs(ctx context.Context, userID, mailboxID string, messageIDs []string) (map[string]uint32, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if len(messageIDs) == 0 {
		return map[string]uint32{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT message_id::text, uid::bigint
FROM imap_message_uid
WHERE mailbox_id = $1::uuid
  AND user_id    = $2::uuid
  AND message_id = ANY($3::uuid[])
`, mailboxID, userID, pq.Array(messageIDs))
	if err != nil {
		return nil, fmt.Errorf("lookup imap message uids: %w", err)
	}
	defer rows.Close()
	result := make(map[string]uint32, len(messageIDs))
	for rows.Next() {
		var id string
		var uid int64
		if err := rows.Scan(&id, &uid); err != nil {
			return nil, fmt.Errorf("scan imap message uid: %w", err)
		}
		result[id] = uint32(uid) //nolint:gosec // uid is bounded [1,4294967295] by DB check
	}
	return result, rows.Err()
}
