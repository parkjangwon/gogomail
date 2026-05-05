package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/imapgw"
)

type IMAPUIDState = imapgw.UIDState
type IMAPMessageUID = imapgw.MessageUID

var ErrIMAPMessageNotActive = errors.New("imap message is not active in mailbox")

const maxIMAPUIDBackfillAuditSample = 10

type IMAPStoredMessage struct {
	Summary     imapgw.MessageSummary
	StoragePath string
}

type imapSequenceQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const (
	imapUIDBackfillDefaultLimit = 500
	imapUIDBackfillMaxLimit     = 5000
)

func (r *Repository) ListIMAPMailboxes(ctx context.Context, userID string) ([]imapgw.Mailbox, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	folders, err := r.ListFolders(ctx, userID)
	if err != nil {
		return nil, err
	}
	mailboxes := make([]imapgw.Mailbox, 0, len(folders))
	for _, folder := range folders {
		state, err := r.EnsureIMAPMailboxState(ctx, userID, folder.ID)
		if err != nil {
			return nil, err
		}
		mailboxes = append(mailboxes, imapMailboxFromFolder(folder, state))
	}
	return mailboxes, nil
}

func (r *Repository) GetIMAPMailbox(ctx context.Context, userID string, mailboxID string) (imapgw.Mailbox, error) {
	if r.db == nil {
		return imapgw.Mailbox{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	if userID == "" {
		return imapgw.Mailbox{}, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
		return imapgw.Mailbox{}, fmt.Errorf("mailbox_id is required")
	}
	mailboxName := normalizeIMAPMailboxLookupName(mailboxID)

	const query = `
SELECT
  f.id::text,
  COALESCE(f.parent_id::text, ''),
  f.name,
  f.full_path,
  f.type,
  COALESCE(f.system_type, ''),
  f.order_index,
  COALESCE(c.total, 0) AS total,
  COALESCE(c.unread, 0) AS unread,
  COALESCE(c.starred, 0) AS starred
FROM folders f
LEFT JOIN (
  SELECT
    folder_id,
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE COALESCE((flags->>'read')::boolean, false) = false) AS unread,
    COUNT(*) FILTER (WHERE COALESCE((flags->>'starred')::boolean, false) = true) AS starred
  FROM messages
  WHERE user_id = $1::uuid
    AND status = 'active'
  GROUP BY folder_id
) c ON c.folder_id = f.id
WHERE f.user_id = $1::uuid
  AND (
    f.id::text = $2
    OR lower(f.name) = $3
    OR lower(trim(both '/' from f.full_path)) = $3
    OR ($3 = 'inbox' AND lower(COALESCE(f.system_type, '')) = 'inbox')
  )
ORDER BY
  CASE WHEN lower(COALESCE(f.system_type, '')) = 'inbox' THEN 0 ELSE 1 END,
  f.full_path,
  f.name
LIMIT 1`

	var folder Folder
	if err := r.db.QueryRowContext(ctx, query, userID, mailboxID, mailboxName).Scan(
		&folder.ID,
		&folder.ParentID,
		&folder.Name,
		&folder.FullPath,
		&folder.Type,
		&folder.SystemType,
		&folder.OrderIndex,
		&folder.Total,
		&folder.Unread,
		&folder.Starred,
	); err != nil {
		if err == sql.ErrNoRows {
			return imapgw.Mailbox{}, fmt.Errorf("%w: %q", imapgw.ErrMailboxNotFound, mailboxID)
		}
		return imapgw.Mailbox{}, fmt.Errorf("get imap mailbox: %w", err)
	}
	state, err := r.EnsureIMAPMailboxState(ctx, userID, folder.ID)
	if err != nil {
		return imapgw.Mailbox{}, err
	}
	return imapMailboxFromFolder(folder, state), nil
}

func normalizeIMAPMailboxLookupName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	value = strings.Trim(value, "/")
	value = strings.Join(strings.Fields(value), " ")
	return strings.ToLower(value)
}

func (r *Repository) ListIMAPMessages(ctx context.Context, userID string, mailboxID string, limit int, afterUID imapgw.UID) ([]imapgw.MessageSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
		return nil, fmt.Errorf("mailbox_id is required")
	}
	limit = NormalizeMessageListLimit(limit)

	if _, err := r.EnsureIMAPMailboxState(ctx, userID, mailboxID); err != nil {
		return nil, err
	}

	const query = `
SELECT
  m.id::text,
  m.folder_id::text,
  COALESCE(m.rfc_message_id, ''),
  m.subject,
  m.from_addr,
  m.from_name,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date,
  m.size,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred,
  COALESCE((m.flags->>'answered')::boolean, false) AS answered,
  COALESCE((m.flags->>'forwarded')::boolean, false) AS forwarded,
  COALESCE((m.flags->>'draft')::boolean, false) AS draft,
  COALESCE((m.flags->>'imap_deleted')::boolean, false) AS deleted,
  m.status,
  i.uid,
  i.modseq
FROM messages m
LEFT JOIN imap_message_uid i ON i.message_id = m.id
WHERE m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'
  AND (i.uid IS NULL OR i.uid > $3)
ORDER BY
  CASE WHEN i.uid IS NULL THEN 1 ELSE 0 END,
  i.uid,
  internal_date,
  m.id
LIMIT $4`

	rows, err := r.db.QueryContext(ctx, query, userID, mailboxID, int64(afterUID), limit)
	if err != nil {
		return nil, fmt.Errorf("list imap messages: %w", err)
	}
	defer rows.Close()

	messages := make([]imapgw.MessageSummary, 0, limit)
	for rows.Next() {
		var row imapMessageRow
		var uid sql.NullInt64
		var modseq sql.NullInt64
		if err := rows.Scan(
			&row.ID,
			&row.MailboxID,
			&row.RFCMessageID,
			&row.Subject,
			&row.FromAddr,
			&row.FromName,
			&row.InternalDate,
			&row.Size,
			&row.Read,
			&row.Starred,
			&row.Answered,
			&row.Forwarded,
			&row.Draft,
			&row.Deleted,
			&row.Status,
			&uid,
			&modseq,
		); err != nil {
			return nil, fmt.Errorf("scan imap message summary: %w", err)
		}

		messageUID := IMAPMessageUID{
			MessageID: imapgw.MessageID(row.ID),
			MailboxID: imapgw.MailboxID(row.MailboxID),
			UID:       imapgw.UID(uid.Int64),
			ModSeq:    uint64(modseq.Int64),
		}
		if !uid.Valid {
			messageUID, err = r.EnsureIMAPMessageUID(ctx, userID, mailboxID, row.ID)
			if err != nil {
				return nil, err
			}
		}
		if messageUID.UID <= afterUID {
			continue
		}
		messages = append(messages, imapMessageFromRow(row, messageUID))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imap message summaries: %w", err)
	}
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].UID < messages[j].UID
	})
	return messages, nil
}

func (r *Repository) GetIMAPMessage(ctx context.Context, userID string, mailboxID string, uid imapgw.UID) (IMAPStoredMessage, error) {
	if r.db == nil {
		return IMAPStoredMessage{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	if userID == "" {
		return IMAPStoredMessage{}, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
		return IMAPStoredMessage{}, fmt.Errorf("mailbox_id is required")
	}
	if uid == 0 {
		return IMAPStoredMessage{}, fmt.Errorf("uid is required")
	}

	const query = `
SELECT
  m.id::text,
  m.folder_id::text,
  COALESCE(m.rfc_message_id, ''),
  m.subject,
  m.from_addr,
  m.from_name,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date,
  m.size,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred,
  COALESCE((m.flags->>'answered')::boolean, false) AS answered,
  COALESCE((m.flags->>'forwarded')::boolean, false) AS forwarded,
  COALESCE((m.flags->>'draft')::boolean, false) AS draft,
  COALESCE((m.flags->>'imap_deleted')::boolean, false) AS deleted,
  m.status,
  m.storage_path,
  i.uid,
  i.modseq
FROM imap_message_uid i
JOIN messages m ON m.id = i.message_id
WHERE i.user_id = $1::uuid
  AND i.mailbox_id = $2::uuid
  AND i.uid = $3
  AND m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'
LIMIT 1`

	var row imapMessageRow
	var storagePath string
	var messageUID IMAPMessageUID
	if err := r.db.QueryRowContext(ctx, query, userID, mailboxID, int64(uid)).Scan(
		&row.ID,
		&row.MailboxID,
		&row.RFCMessageID,
		&row.Subject,
		&row.FromAddr,
		&row.FromName,
		&row.InternalDate,
		&row.Size,
		&row.Read,
		&row.Starred,
		&row.Answered,
		&row.Forwarded,
		&row.Draft,
		&row.Deleted,
		&row.Status,
		&storagePath,
		&messageUID.UID,
		&messageUID.ModSeq,
	); err != nil {
		if err == sql.ErrNoRows {
			return IMAPStoredMessage{}, fmt.Errorf("imap message uid %d not found", uid)
		}
		return IMAPStoredMessage{}, fmt.Errorf("get imap message: %w", err)
	}
	messageUID.MessageID = imapgw.MessageID(row.ID)
	messageUID.MailboxID = imapgw.MailboxID(row.MailboxID)
	if err := imapgw.ValidateMessageUID(messageUID); err != nil {
		return IMAPStoredMessage{}, err
	}
	summary := imapMessageFromRow(row, messageUID)
	sequenceNumber, err := imapSequenceNumberForUID(ctx, r.db, userID, mailboxID, uid)
	if err != nil {
		return IMAPStoredMessage{}, err
	}
	summary.SequenceNumber = sequenceNumber
	return IMAPStoredMessage{
		Summary:     summary,
		StoragePath: strings.TrimSpace(storagePath),
	}, nil
}

func (r *Repository) StoreIMAPFlags(ctx context.Context, userID string, mailboxID string, uids []imapgw.UID, flags imapgw.MessageFlags, mode imapgw.StoreFlagsMode, unchangedSince uint64) ([]imapgw.MessageSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
		return nil, fmt.Errorf("mailbox_id is required")
	}
	if len(uids) == 0 {
		return nil, fmt.Errorf("uids are required")
	}
	if len(uids) > 500 {
		return nil, fmt.Errorf("too many uids")
	}
	for _, uid := range uids {
		if uid == 0 {
			return nil, fmt.Errorf("uid must not be zero")
		}
	}
	changes, err := newIMAPStoreFlagChanges(flags, mode)
	if err != nil {
		return nil, err
	}

	if _, err := r.EnsureIMAPMailboxState(ctx, userID, mailboxID); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin imap store flags transaction: %w", err)
	}
	defer tx.Rollback()

	var highestModSeq uint64
	const lockMailbox = `
SELECT highest_modseq
FROM imap_mailbox_state
WHERE mailbox_id = $1::uuid
  AND user_id = $2::uuid
FOR UPDATE`
	if err := tx.QueryRowContext(ctx, lockMailbox, mailboxID, userID).Scan(&highestModSeq); err != nil {
		return nil, fmt.Errorf("lock imap mailbox state: %w", err)
	}

	type storeCandidate struct {
		uid        imapgw.UID
		row        imapMessageRow
		messageUID IMAPMessageUID
	}
	candidates := make([]storeCandidate, 0, len(uids))
	for _, uid := range uids {
		row, messageUID, err := scanIMAPMessageByUID(ctx, tx, userID, mailboxID, uid)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, storeCandidate{uid: uid, row: row, messageUID: messageUID})
	}

	summaries := make([]imapgw.MessageSummary, 0, len(candidates))
	modified := make([]imapgw.UID, 0)
	changedAny := false
	for _, candidate := range candidates {
		row := candidate.row
		messageUID := candidate.messageUID
		if unchangedSince > 0 && messageUID.ModSeq > unchangedSince {
			modified = append(modified, candidate.uid)
			continue
		}
		next, changed := applyIMAPStoreFlagChanges(row, changes)
		if changed {
			changedAny = true
			highestModSeq++
			messageUID.ModSeq = highestModSeq
			if err := updateIMAPMessageFlags(ctx, tx, row.ID, messageUID.ModSeq, next); err != nil {
				return nil, err
			}
			row = next
		}
		summary := imapMessageFromRow(row, messageUID)
		sequenceNumber, err := imapSequenceNumberForUID(ctx, tx, userID, mailboxID, candidate.uid)
		if err != nil {
			return nil, err
		}
		summary.SequenceNumber = sequenceNumber
		summaries = append(summaries, summary)
	}
	if changedAny {
		if err := updateIMAPMailboxModSeq(ctx, tx, mailboxID, highestModSeq); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit imap store flags transaction: %w", err)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].UID < summaries[j].UID
	})
	if len(modified) > 0 {
		return summaries, &imapgw.StoreModifiedError{UIDs: modified, Summaries: summaries}
	}
	return summaries, nil
}

func (r *Repository) CopyIMAPMessages(ctx context.Context, userID string, sourceMailboxID string, destMailboxID string, uids []imapgw.UID) ([]imapgw.MessageSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	sourceMailboxID = strings.TrimSpace(sourceMailboxID)
	destMailboxID = strings.TrimSpace(destMailboxID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if sourceMailboxID == "" {
		return nil, fmt.Errorf("source_mailbox_id is required")
	}
	if destMailboxID == "" {
		return nil, fmt.Errorf("dest_mailbox_id is required")
	}
	if len(uids) == 0 {
		return nil, nil
	}
	if len(uids) > 500 {
		return nil, fmt.Errorf("too many uids")
	}
	for _, uid := range uids {
		if uid == 0 {
			return nil, fmt.Errorf("uid must not be zero")
		}
	}
	rawUIDs, err := json.Marshal(uids)
	if err != nil {
		return nil, fmt.Errorf("encode imap copy uids: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin imap copy transaction: %w", err)
	}
	defer tx.Rollback()

	var destExists bool
	if err := tx.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM folders
  WHERE id = $1::uuid
    AND user_id = $2::uuid
)`, destMailboxID, userID).Scan(&destExists); err != nil {
		return nil, fmt.Errorf("validate imap copy destination mailbox: %w", err)
	}
	if !destExists {
		return nil, fmt.Errorf("%w: destination %q", imapgw.ErrMailboxNotFound, destMailboxID)
	}
	const ensureState = `
INSERT INTO imap_mailbox_state (mailbox_id, user_id)
SELECT id, user_id
FROM folders
WHERE id = $1::uuid
  AND user_id = $2::uuid
ON CONFLICT (mailbox_id) DO NOTHING`
	if _, err := tx.ExecContext(ctx, ensureState, destMailboxID, userID); err != nil {
		return nil, fmt.Errorf("ensure imap destination mailbox state: %w", err)
	}

	var totalSize int64
	const totalQuery = `
WITH input AS (
  SELECT value::bigint AS uid
  FROM jsonb_array_elements_text($4::jsonb)
),
source AS (
  SELECT m.id, m.size
  FROM input
  JOIN imap_message_uid i
    ON i.uid = input.uid
   AND i.user_id = $1::uuid
   AND i.mailbox_id = $2::uuid
  JOIN messages m
    ON m.id = i.message_id
   AND m.user_id = $1::uuid
   AND m.folder_id = $2::uuid
   AND m.status = 'active'
),
source_bytes AS (
  SELECT
    source.id,
    source.size + COALESCE(SUM(a.size), 0) AS copied_size
  FROM source
  LEFT JOIN attachments a ON a.message_id = source.id
  GROUP BY source.id, source.size
)
SELECT COALESCE(SUM(copied_size), 0)
FROM source_bytes
WHERE EXISTS (
  SELECT 1
  FROM folders f
  WHERE f.id = $3::uuid
    AND f.user_id = $1::uuid
)`
	if err := tx.QueryRowContext(ctx, totalQuery, userID, sourceMailboxID, destMailboxID, string(rawUIDs)).Scan(&totalSize); err != nil {
		return nil, fmt.Errorf("sum imap copy message sizes: %w", err)
	}
	if err := checkAndIncrementUserQuota(ctx, tx, userID, totalSize); err != nil {
		return nil, err
	}

	const copyQuery = `
WITH input AS (
  SELECT value::bigint AS uid, ordinality
  FROM jsonb_array_elements_text($4::jsonb) WITH ORDINALITY
),
locked_state AS (
  SELECT mailbox_id, user_id, uidnext, highest_modseq
  FROM imap_mailbox_state
  WHERE mailbox_id = $3::uuid
    AND user_id = $1::uuid
  FOR UPDATE
),
source AS (
  SELECT
    gen_random_uuid() AS new_id,
    m.id AS source_id,
    m.tenant_id,
    m.domain_id,
    m.user_id,
    m.rfc_message_id,
    m.in_reply_to,
    m.thread_id,
    m.subject,
    m.from_addr,
    m.from_name,
    m.to_addrs,
    m.cc_addrs,
    m.bcc_addrs,
    m.received_at,
    m.sent_at,
    m.size,
    m.has_attachment,
    m.flags,
    m.spam_score,
    m.storage_path,
    m.dek_encrypted,
    m.legal_hold,
    m.compose_intent,
    m.source_message_id,
    m.draft_text_body,
    row_number() OVER (ORDER BY input.ordinality) AS rn
  FROM input
  JOIN imap_message_uid i
    ON i.uid = input.uid
   AND i.user_id = $1::uuid
   AND i.mailbox_id = $2::uuid
  JOIN messages m
    ON m.id = i.message_id
   AND m.user_id = $1::uuid
   AND m.folder_id = $2::uuid
   AND m.status = 'active'
),
inserted_messages AS (
  INSERT INTO messages (
    id,
    tenant_id,
    domain_id,
    user_id,
    folder_id,
    rfc_message_id,
    in_reply_to,
    thread_id,
    subject,
    from_addr,
    from_name,
    to_addrs,
    cc_addrs,
    bcc_addrs,
    received_at,
    sent_at,
    size,
    has_attachment,
    flags,
    spam_score,
    storage_path,
    dek_encrypted,
    status,
    legal_hold,
    compose_intent,
    source_message_id,
    draft_text_body
  )
  SELECT
    new_id,
    tenant_id,
    domain_id,
    user_id,
    $3::uuid,
    rfc_message_id,
    in_reply_to,
    thread_id,
    subject,
    from_addr,
    from_name,
    to_addrs,
    cc_addrs,
    bcc_addrs,
    received_at,
    sent_at,
    size,
    has_attachment,
    flags,
    spam_score,
    storage_path,
    dek_encrypted,
    'active',
    legal_hold,
    compose_intent,
    source_message_id,
    draft_text_body
  FROM source
  RETURNING id
),
inserted_attachments AS (
  INSERT INTO attachments (
    message_id,
    user_id,
    draft_id,
    upload_id,
    storage_path,
    filename,
    size,
    mime_type,
    status
  )
  SELECT
    source.new_id,
    COALESCE(a.user_id, $1::uuid),
    NULL,
    'imap-copy/' || source.new_id::text || '/' || a.id::text,
    a.storage_path,
    a.filename,
    a.size,
    a.mime_type,
    a.status
  FROM source
  JOIN attachments a ON a.message_id = source.source_id
  RETURNING 1
),
inserted_uids AS (
  INSERT INTO imap_message_uid (message_id, mailbox_id, user_id, uid, modseq)
  SELECT
    source.new_id,
    locked_state.mailbox_id,
    locked_state.user_id,
    locked_state.uidnext + source.rn - 1,
    locked_state.highest_modseq + source.rn
  FROM source
  CROSS JOIN locked_state
  RETURNING message_id, mailbox_id, user_id, uid, modseq
),
updated_state AS (
  UPDATE imap_mailbox_state
  SET uidnext = uidnext + (SELECT COUNT(*) FROM source),
      highest_modseq = highest_modseq + (SELECT COUNT(*) FROM source),
      updated_at = CASE WHEN EXISTS (SELECT 1 FROM source) THEN now() ELSE updated_at END
  WHERE mailbox_id = $3::uuid
    AND user_id = $1::uuid
  RETURNING 1
)
SELECT
  source.new_id::text,
  $3::text AS mailbox_id,
  COALESCE(source.rfc_message_id, ''),
  source.subject,
  source.from_addr,
  source.from_name,
  COALESCE(source.received_at, source.sent_at, now()) AS internal_date,
  source.size,
  COALESCE((source.flags->>'read')::boolean, false) AS read,
  COALESCE((source.flags->>'starred')::boolean, false) AS starred,
  COALESCE((source.flags->>'answered')::boolean, false) AS answered,
  COALESCE((source.flags->>'forwarded')::boolean, false) AS forwarded,
  COALESCE((source.flags->>'draft')::boolean, false) AS draft,
  COALESCE((source.flags->>'imap_deleted')::boolean, false) AS deleted,
  'active' AS status,
  inserted_uids.uid,
  inserted_uids.modseq,
  (SELECT COUNT(*) FROM inserted_attachments) AS attachment_copy_count
FROM source
JOIN inserted_messages ON inserted_messages.id = source.new_id
JOIN inserted_uids ON inserted_uids.message_id = source.new_id
ORDER BY source.rn`
	rows, err := tx.QueryContext(ctx, copyQuery, userID, sourceMailboxID, destMailboxID, string(rawUIDs))
	if err != nil {
		return nil, fmt.Errorf("copy imap messages: %w", err)
	}
	defer rows.Close()

	summaries := make([]imapgw.MessageSummary, 0, len(uids))
	for rows.Next() {
		var row imapMessageRow
		var messageUID IMAPMessageUID
		var attachmentCopyCount int64
		if err := rows.Scan(
			&row.ID,
			&row.MailboxID,
			&row.RFCMessageID,
			&row.Subject,
			&row.FromAddr,
			&row.FromName,
			&row.InternalDate,
			&row.Size,
			&row.Read,
			&row.Starred,
			&row.Answered,
			&row.Forwarded,
			&row.Draft,
			&row.Deleted,
			&row.Status,
			&messageUID.UID,
			&messageUID.ModSeq,
			&attachmentCopyCount,
		); err != nil {
			return nil, fmt.Errorf("scan copied imap message: %w", err)
		}
		_ = attachmentCopyCount
		messageUID.MessageID = imapgw.MessageID(row.ID)
		messageUID.MailboxID = imapgw.MailboxID(row.MailboxID)
		summary := imapMessageFromRow(row, messageUID)
		sequenceNumber, err := imapSequenceNumberForUID(ctx, tx, userID, destMailboxID, messageUID.UID)
		if err != nil {
			return nil, err
		}
		summary.SequenceNumber = sequenceNumber
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate copied imap messages: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit imap copy transaction: %w", err)
	}
	return summaries, nil
}

func (r *Repository) ExpungeIMAPMessages(ctx context.Context, userID string, mailboxID string, uids []imapgw.UID) ([]imapgw.MessageSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
		return nil, fmt.Errorf("mailbox_id is required")
	}
	if len(uids) > 500 {
		return nil, fmt.Errorf("too many uids")
	}
	for _, uid := range uids {
		if uid == 0 {
			return nil, fmt.Errorf("uid must not be zero")
		}
	}
	rawUIDs, err := json.Marshal(uids)
	if err != nil {
		return nil, fmt.Errorf("encode imap expunge uids: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin imap expunge transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
WITH input AS (
  SELECT value::bigint AS uid
  FROM jsonb_array_elements_text($4::jsonb)
)
SELECT
  m.id::text,
  m.folder_id::text,
  COALESCE(m.rfc_message_id, ''),
  m.subject,
  m.from_addr,
  m.from_name,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date,
  m.size,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred,
  COALESCE((m.flags->>'answered')::boolean, false) AS answered,
  COALESCE((m.flags->>'forwarded')::boolean, false) AS forwarded,
  COALESCE((m.flags->>'draft')::boolean, false) AS draft,
  COALESCE((m.flags->>'imap_deleted')::boolean, false) AS deleted,
  m.status,
  i.uid,
  i.modseq
FROM imap_message_uid i
JOIN messages m ON m.id = i.message_id
WHERE i.user_id = $1::uuid
  AND i.mailbox_id = $2::uuid
  AND m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'
  AND COALESCE((m.flags->>'imap_deleted')::boolean, false) = true
  AND ($3::bool = false OR i.uid IN (SELECT uid FROM input))
ORDER BY i.uid
FOR UPDATE OF i, m`
	rows, err := tx.QueryContext(ctx, query, userID, mailboxID, len(uids) > 0, string(rawUIDs))
	if err != nil {
		return nil, fmt.Errorf("list imap expunge messages: %w", err)
	}
	var messageIDs []string
	var totalSize int64
	summaries := make([]imapgw.MessageSummary, 0)
	for rows.Next() {
		var row imapMessageRow
		var messageUID IMAPMessageUID
		if err := rows.Scan(
			&row.ID,
			&row.MailboxID,
			&row.RFCMessageID,
			&row.Subject,
			&row.FromAddr,
			&row.FromName,
			&row.InternalDate,
			&row.Size,
			&row.Read,
			&row.Starred,
			&row.Answered,
			&row.Forwarded,
			&row.Draft,
			&row.Deleted,
			&row.Status,
			&messageUID.UID,
			&messageUID.ModSeq,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan imap expunge message: %w", err)
		}
		messageUID.MessageID = imapgw.MessageID(row.ID)
		messageUID.MailboxID = imapgw.MailboxID(row.MailboxID)
		summary := imapMessageFromRow(row, messageUID)
		sequenceNumber, err := imapSequenceNumberForUID(ctx, tx, userID, mailboxID, messageUID.UID)
		if err != nil {
			rows.Close()
			return nil, err
		}
		summary.SequenceNumber = sequenceNumber
		summaries = append(summaries, summary)
		messageIDs = append(messageIDs, row.ID)
		totalSize += row.Size
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close imap expunge rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imap expunge rows: %w", err)
	}
	if len(messageIDs) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit empty imap expunge transaction: %w", err)
		}
		return nil, nil
	}
	rawMessageIDs, err := json.Marshal(messageIDs)
	if err != nil {
		return nil, fmt.Errorf("encode imap expunge message ids: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET status = 'deleted',
    deleted_at = now(),
    updated_at = now()
WHERE user_id = $1::uuid
  AND id IN (SELECT value::uuid FROM jsonb_array_elements_text($2::jsonb))
  AND status = 'active'`, userID, string(rawMessageIDs)); err != nil {
		return nil, fmt.Errorf("expunge imap messages: %w", err)
	}
	if err := deleteIMAPUIDRowsForMessages(ctx, tx, userID, messageIDs); err != nil {
		return nil, err
	}
	if err := decrementUserQuota(ctx, tx, userID, totalSize); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit imap expunge transaction: %w", err)
	}
	return summaries, nil
}

func (r *Repository) MoveIMAPMessages(ctx context.Context, userID string, sourceMailboxID string, destMailboxID string, uids []imapgw.UID) ([]imapgw.MoveMessageResult, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	sourceMailboxID = strings.TrimSpace(sourceMailboxID)
	destMailboxID = strings.TrimSpace(destMailboxID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if sourceMailboxID == "" {
		return nil, fmt.Errorf("source_mailbox_id is required")
	}
	if destMailboxID == "" {
		return nil, fmt.Errorf("dest_mailbox_id is required")
	}
	if sourceMailboxID == destMailboxID {
		return r.moveIMAPMessagesWithinMailbox(ctx, userID, sourceMailboxID, uids)
	}
	if len(uids) == 0 {
		return nil, nil
	}
	if len(uids) > 500 {
		return nil, fmt.Errorf("too many uids")
	}
	for _, uid := range uids {
		if uid == 0 {
			return nil, fmt.Errorf("uid must not be zero")
		}
	}
	rawUIDs, err := json.Marshal(uids)
	if err != nil {
		return nil, fmt.Errorf("encode imap move uids: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin imap move transaction: %w", err)
	}
	defer tx.Rollback()

	var destExists bool
	if err := tx.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM folders
  WHERE id = $1::uuid
    AND user_id = $2::uuid
)`, destMailboxID, userID).Scan(&destExists); err != nil {
		return nil, fmt.Errorf("validate imap move destination mailbox: %w", err)
	}
	if !destExists {
		return nil, fmt.Errorf("%w: destination %q", imapgw.ErrMailboxNotFound, destMailboxID)
	}
	const ensureState = `
INSERT INTO imap_mailbox_state (mailbox_id, user_id)
SELECT id, user_id
FROM folders
WHERE id = $1::uuid
  AND user_id = $2::uuid
ON CONFLICT (mailbox_id) DO NOTHING`
	if _, err := tx.ExecContext(ctx, ensureState, destMailboxID, userID); err != nil {
		return nil, fmt.Errorf("ensure imap move destination mailbox state: %w", err)
	}

	type moveMailboxState struct {
		mailboxID     string
		uidNext       imapgw.UID
		highestModSeq uint64
	}
	states := make(map[string]moveMailboxState, 2)
	const lockMoveStates = `
SELECT mailbox_id::text, uidnext, highest_modseq
FROM imap_mailbox_state
WHERE user_id = $1::uuid
  AND mailbox_id IN ($2::uuid, $3::uuid)
ORDER BY mailbox_id
FOR UPDATE`
	stateRows, err := tx.QueryContext(ctx, lockMoveStates, userID, sourceMailboxID, destMailboxID)
	if err != nil {
		return nil, fmt.Errorf("lock imap move mailbox states: %w", err)
	}
	for stateRows.Next() {
		var state moveMailboxState
		if err := stateRows.Scan(&state.mailboxID, &state.uidNext, &state.highestModSeq); err != nil {
			stateRows.Close()
			return nil, fmt.Errorf("scan imap move mailbox state: %w", err)
		}
		states[state.mailboxID] = state
	}
	if err := stateRows.Close(); err != nil {
		return nil, fmt.Errorf("close imap move mailbox states: %w", err)
	}
	if err := stateRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imap move mailbox states: %w", err)
	}
	sourceState, ok := states[sourceMailboxID]
	if !ok {
		return nil, fmt.Errorf("source imap mailbox state is required")
	}
	destState, ok := states[destMailboxID]
	if !ok {
		return nil, fmt.Errorf("destination imap mailbox state is required")
	}

	const query = `
WITH input AS (
  SELECT value::bigint AS uid, ordinality
  FROM jsonb_array_elements_text($3::jsonb) WITH ORDINALITY
)
SELECT
  m.id::text,
  m.folder_id::text,
  COALESCE(m.rfc_message_id, ''),
  m.subject,
  m.from_addr,
  m.from_name,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date,
  m.size,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred,
  COALESCE((m.flags->>'answered')::boolean, false) AS answered,
  COALESCE((m.flags->>'forwarded')::boolean, false) AS forwarded,
  COALESCE((m.flags->>'draft')::boolean, false) AS draft,
  COALESCE((m.flags->>'imap_deleted')::boolean, false) AS deleted,
  m.status,
  i.uid,
  i.modseq
FROM input
JOIN imap_message_uid i
  ON i.uid = input.uid
 AND i.user_id = $1::uuid
 AND i.mailbox_id = $2::uuid
JOIN messages m
  ON m.id = i.message_id
 AND m.user_id = $1::uuid
 AND m.folder_id = $2::uuid
 AND m.status = 'active'
ORDER BY input.ordinality
FOR UPDATE OF i, m`
	rows, err := tx.QueryContext(ctx, query, userID, sourceMailboxID, string(rawUIDs))
	if err != nil {
		return nil, fmt.Errorf("list imap move messages: %w", err)
	}
	var messageIDs []string
	sourceRows := make([]imapMessageRow, 0, len(uids))
	sourceSummaries := make([]imapgw.MessageSummary, 0, len(uids))
	for rows.Next() {
		var row imapMessageRow
		var messageUID IMAPMessageUID
		if err := rows.Scan(
			&row.ID,
			&row.MailboxID,
			&row.RFCMessageID,
			&row.Subject,
			&row.FromAddr,
			&row.FromName,
			&row.InternalDate,
			&row.Size,
			&row.Read,
			&row.Starred,
			&row.Answered,
			&row.Forwarded,
			&row.Draft,
			&row.Deleted,
			&row.Status,
			&messageUID.UID,
			&messageUID.ModSeq,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan imap move message: %w", err)
		}
		messageUID.MessageID = imapgw.MessageID(row.ID)
		messageUID.MailboxID = imapgw.MailboxID(row.MailboxID)
		summary := imapMessageFromRow(row, messageUID)
		sequenceNumber, err := imapSequenceNumberForUID(ctx, tx, userID, sourceMailboxID, messageUID.UID)
		if err != nil {
			rows.Close()
			return nil, err
		}
		summary.SequenceNumber = sequenceNumber
		sourceRows = append(sourceRows, row)
		sourceSummaries = append(sourceSummaries, summary)
		messageIDs = append(messageIDs, row.ID)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close imap move rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imap move rows: %w", err)
	}
	if len(messageIDs) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit empty imap move transaction: %w", err)
		}
		return nil, nil
	}
	if uint64(destState.uidNext)+uint64(len(messageIDs))-1 > math.MaxUint32 {
		return nil, fmt.Errorf("imap destination uidnext exhausted")
	}
	rawMessageIDs, err := json.Marshal(messageIDs)
	if err != nil {
		return nil, fmt.Errorf("encode imap move message ids: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET folder_id = $3::uuid,
    updated_at = now()
WHERE user_id = $1::uuid
  AND id IN (SELECT value::uuid FROM jsonb_array_elements_text($2::jsonb))
  AND status = 'active'`, userID, string(rawMessageIDs), destMailboxID); err != nil {
		return nil, fmt.Errorf("move imap messages: %w", err)
	}

	results := make([]imapgw.MoveMessageResult, 0, len(sourceSummaries))
	for i, source := range sourceSummaries {
		sourceHighestModSeq := sourceState.highestModSeq + uint64(i) + 1
		destUID := destState.uidNext + imapgw.UID(i)
		destModSeq := destState.highestModSeq + uint64(i) + 1
		const updateUID = `
UPDATE imap_message_uid
SET mailbox_id = $4::uuid,
    uid = $5,
    modseq = $6,
    updated_at = now()
WHERE message_id = $1::uuid
  AND user_id = $2::uuid
  AND mailbox_id = $3::uuid`
		res, err := tx.ExecContext(ctx, updateUID, string(source.ID), userID, sourceMailboxID, destMailboxID, int64(destUID), int64(destModSeq))
		if err != nil {
			return nil, fmt.Errorf("move imap message uid: %w", err)
		}
		if affected, err := res.RowsAffected(); err != nil {
			return nil, fmt.Errorf("read moved imap uid count: %w", err)
		} else if affected != 1 {
			return nil, fmt.Errorf("move imap message uid affected %d rows", affected)
		}

		destUIDRecord := IMAPMessageUID{
			MessageID: imapgw.MessageID(sourceRows[i].ID),
			MailboxID: imapgw.MailboxID(destMailboxID),
			UID:       destUID,
			ModSeq:    destModSeq,
		}
		destRow := sourceRows[i]
		destRow.MailboxID = destMailboxID
		destination := imapMessageFromRow(destRow, destUIDRecord)
		sequenceNumber, err := imapSequenceNumberForUID(ctx, tx, userID, destMailboxID, destUID)
		if err != nil {
			return nil, err
		}
		destination.SequenceNumber = sequenceNumber
		results = append(results, imapgw.MoveMessageResult{Source: source, Destination: destination, SourceHighestModSeq: sourceHighestModSeq})
	}
	const updateSourceState = `
UPDATE imap_mailbox_state
SET highest_modseq = $3,
    updated_at = now()
WHERE mailbox_id = $1::uuid
  AND user_id = $2::uuid`
	if _, err := tx.ExecContext(ctx, updateSourceState, sourceMailboxID, userID, int64(sourceState.highestModSeq+uint64(len(messageIDs)))); err != nil {
		return nil, fmt.Errorf("update imap move source mailbox state: %w", err)
	}
	const updateDestState = `
UPDATE imap_mailbox_state
SET uidnext = $3,
    highest_modseq = $4,
    updated_at = now()
WHERE mailbox_id = $1::uuid
  AND user_id = $2::uuid`
	if _, err := tx.ExecContext(ctx, updateDestState, destMailboxID, userID, int64(destState.uidNext+imapgw.UID(len(messageIDs))), int64(destState.highestModSeq+uint64(len(messageIDs)))); err != nil {
		return nil, fmt.Errorf("update imap move destination mailbox state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit imap move transaction: %w", err)
	}
	return results, nil
}

func (r *Repository) moveIMAPMessagesWithinMailbox(ctx context.Context, userID string, mailboxID string, uids []imapgw.UID) ([]imapgw.MoveMessageResult, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	if len(uids) > 500 {
		return nil, fmt.Errorf("too many uids")
	}
	for _, uid := range uids {
		if uid == 0 {
			return nil, fmt.Errorf("uid must not be zero")
		}
	}
	rawUIDs, err := json.Marshal(uids)
	if err != nil {
		return nil, fmt.Errorf("encode imap same-mailbox move uids: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin imap same-mailbox move transaction: %w", err)
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
		return nil, fmt.Errorf("ensure imap same-mailbox move state: %w", err)
	}

	const query = `
WITH input AS (
  SELECT value::bigint AS uid, ordinality
  FROM jsonb_array_elements_text($3::jsonb) WITH ORDINALITY
),
locked_state AS (
  SELECT mailbox_id, user_id, uidnext, highest_modseq
  FROM imap_mailbox_state
  WHERE mailbox_id = $2::uuid
    AND user_id = $1::uuid
  FOR UPDATE
),
source AS (
  SELECT
    gen_random_uuid() AS new_id,
    m.id AS source_id,
    m.tenant_id,
    m.domain_id,
    m.user_id,
    m.rfc_message_id,
    m.in_reply_to,
    m.thread_id,
    m.subject,
    m.from_addr,
    m.from_name,
    m.to_addrs,
    m.cc_addrs,
    m.bcc_addrs,
    m.received_at,
    m.sent_at,
    m.draft_updated_at,
    m.created_at,
    m.size,
    m.has_attachment,
    m.flags,
    m.spam_score,
    m.storage_path,
    m.dek_encrypted,
    m.legal_hold,
    m.compose_intent,
    m.source_message_id,
    m.draft_text_body,
    i.uid AS source_uid,
    i.modseq AS source_modseq,
    (
      SELECT COUNT(*)
      FROM imap_message_uid seq
      WHERE seq.user_id = $1::uuid
        AND seq.mailbox_id = $2::uuid
        AND seq.uid <= i.uid
    ) AS source_sequence_number,
    row_number() OVER (ORDER BY input.ordinality) AS rn,
    COUNT(*) OVER () AS moved_count
  FROM input
  JOIN imap_message_uid i
    ON i.uid = input.uid
   AND i.user_id = $1::uuid
   AND i.mailbox_id = $2::uuid
  JOIN messages m
    ON m.id = i.message_id
   AND m.user_id = $1::uuid
   AND m.folder_id = $2::uuid
   AND m.status = 'active'
),
inserted_messages AS (
  INSERT INTO messages (
    id,
    tenant_id,
    domain_id,
    user_id,
    folder_id,
    rfc_message_id,
    in_reply_to,
    thread_id,
    subject,
    from_addr,
    from_name,
    to_addrs,
    cc_addrs,
    bcc_addrs,
    received_at,
    sent_at,
    size,
    has_attachment,
    flags,
    spam_score,
    storage_path,
    dek_encrypted,
    status,
    legal_hold,
    compose_intent,
    source_message_id,
    draft_text_body
  )
  SELECT
    new_id,
    tenant_id,
    domain_id,
    user_id,
    $2::uuid,
    rfc_message_id,
    in_reply_to,
    thread_id,
    subject,
    from_addr,
    from_name,
    to_addrs,
    cc_addrs,
    bcc_addrs,
    received_at,
    sent_at,
    size,
    has_attachment,
    flags,
    spam_score,
    storage_path,
    dek_encrypted,
    'active',
    legal_hold,
    compose_intent,
    source_message_id,
    draft_text_body
  FROM source
  RETURNING id
),
inserted_attachments AS (
  INSERT INTO attachments (
    message_id,
    user_id,
    draft_id,
    upload_id,
    storage_path,
    filename,
    size,
    mime_type,
    status
  )
  SELECT
    source.new_id,
    COALESCE(a.user_id, $1::uuid),
    NULL,
    'imap-move/' || source.new_id::text || '/' || a.id::text,
    a.storage_path,
    a.filename,
    a.size,
    a.mime_type,
    a.status
  FROM source
  JOIN attachments a ON a.message_id = source.source_id
  RETURNING 1
),
inserted_uids AS (
  INSERT INTO imap_message_uid (message_id, mailbox_id, user_id, uid, modseq)
  SELECT
    source.new_id,
    locked_state.mailbox_id,
    locked_state.user_id,
    locked_state.uidnext + source.rn - 1,
    locked_state.highest_modseq + source.rn
  FROM source
  CROSS JOIN locked_state
  RETURNING message_id, mailbox_id, user_id, uid, modseq
),
deleted_messages AS (
  UPDATE messages
  SET status = 'deleted',
      updated_at = now()
  WHERE user_id = $1::uuid
    AND id IN (SELECT source_id FROM source)
    AND status = 'active'
  RETURNING id
),
deleted_uids AS (
  DELETE FROM imap_message_uid
  WHERE user_id = $1::uuid
    AND message_id IN (SELECT source_id FROM source)
  RETURNING 1
),
updated_state AS (
  UPDATE imap_mailbox_state
  SET uidnext = uidnext + (SELECT COUNT(*) FROM source),
      highest_modseq = highest_modseq + (SELECT COUNT(*) FROM source),
      updated_at = CASE WHEN EXISTS (SELECT 1 FROM source) THEN now() ELSE updated_at END
  WHERE mailbox_id = $2::uuid
    AND user_id = $1::uuid
  RETURNING highest_modseq
)
SELECT
  source.source_id::text,
  $2::text AS source_mailbox_id,
  COALESCE(source.rfc_message_id, ''),
  source.subject,
  source.from_addr,
  source.from_name,
  COALESCE(source.received_at, source.sent_at, source.draft_updated_at, source.created_at) AS source_internal_date,
  source.size,
  COALESCE((source.flags->>'read')::boolean, false) AS source_read,
  COALESCE((source.flags->>'starred')::boolean, false) AS source_starred,
  COALESCE((source.flags->>'answered')::boolean, false) AS source_answered,
  COALESCE((source.flags->>'forwarded')::boolean, false) AS source_forwarded,
  COALESCE((source.flags->>'draft')::boolean, false) AS source_draft,
  COALESCE((source.flags->>'imap_deleted')::boolean, false) AS source_deleted,
  'active' AS source_status,
  source.source_uid,
  source.source_modseq,
  source.source_sequence_number,
  source.new_id::text,
  $2::text AS dest_mailbox_id,
  inserted_uids.uid,
  inserted_uids.modseq,
  (SELECT highest_modseq FROM updated_state) AS source_highest_modseq,
  (SELECT COUNT(*) FROM inserted_attachments) AS attachment_copy_count
FROM source
JOIN inserted_messages ON inserted_messages.id = source.new_id
JOIN inserted_uids ON inserted_uids.message_id = source.new_id
WHERE EXISTS (SELECT 1 FROM deleted_messages)
  AND EXISTS (SELECT 1 FROM deleted_uids)
ORDER BY source.rn`
	rows, err := tx.QueryContext(ctx, query, userID, mailboxID, string(rawUIDs))
	if err != nil {
		return nil, fmt.Errorf("move imap messages within mailbox: %w", err)
	}
	defer rows.Close()

	results := make([]imapgw.MoveMessageResult, 0, len(uids))
	for rows.Next() {
		var sourceRow imapMessageRow
		var sourceUID IMAPMessageUID
		var sourceSequenceNumber int64
		var destMessageID string
		var destMailboxID string
		var destUID IMAPMessageUID
		var sourceHighestModSeq uint64
		var attachmentCopyCount int64
		if err := rows.Scan(
			&sourceRow.ID,
			&sourceRow.MailboxID,
			&sourceRow.RFCMessageID,
			&sourceRow.Subject,
			&sourceRow.FromAddr,
			&sourceRow.FromName,
			&sourceRow.InternalDate,
			&sourceRow.Size,
			&sourceRow.Read,
			&sourceRow.Starred,
			&sourceRow.Answered,
			&sourceRow.Forwarded,
			&sourceRow.Draft,
			&sourceRow.Deleted,
			&sourceRow.Status,
			&sourceUID.UID,
			&sourceUID.ModSeq,
			&sourceSequenceNumber,
			&destMessageID,
			&destMailboxID,
			&destUID.UID,
			&destUID.ModSeq,
			&sourceHighestModSeq,
			&attachmentCopyCount,
		); err != nil {
			return nil, fmt.Errorf("scan same-mailbox moved imap message: %w", err)
		}
		_ = attachmentCopyCount
		sourceUID.MessageID = imapgw.MessageID(sourceRow.ID)
		sourceUID.MailboxID = imapgw.MailboxID(sourceRow.MailboxID)
		source := imapMessageFromRow(sourceRow, sourceUID)
		if sourceSequenceNumber <= 0 || sourceSequenceNumber > math.MaxUint32 {
			return nil, fmt.Errorf("imap same-mailbox move source sequence number is unavailable")
		}
		source.SequenceNumber = uint32(sourceSequenceNumber)

		destUID.MessageID = imapgw.MessageID(destMessageID)
		destUID.MailboxID = imapgw.MailboxID(destMailboxID)
		destRow := sourceRow
		destRow.ID = destMessageID
		destRow.MailboxID = destMailboxID
		destination := imapMessageFromRow(destRow, destUID)
		destSequenceNumber, err := imapSequenceNumberForUID(ctx, tx, userID, mailboxID, destUID.UID)
		if err != nil {
			return nil, err
		}
		destination.SequenceNumber = destSequenceNumber
		results = append(results, imapgw.MoveMessageResult{Source: source, Destination: destination, SourceHighestModSeq: sourceHighestModSeq})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate same-mailbox moved imap messages: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit imap same-mailbox move transaction: %w", err)
	}
	return results, nil
}

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
	rawIDs, err := json.Marshal(messageIDs)
	if err != nil {
		return nil, fmt.Errorf("encode imap uid message ids: %w", err)
	}

	const query = `
WITH input AS (
  SELECT value::uuid AS message_id, ordinality
  FROM jsonb_array_elements_text($2::jsonb) WITH ORDINALITY
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
	rows, err := r.db.QueryContext(ctx, query, userID, string(rawIDs))
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

func (r *Repository) BackfillIMAPMailboxUIDs(ctx context.Context, userID string, mailboxID string, limit int) ([]IMAPMessageUID, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
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
		MailboxID:     strings.TrimSpace(mailboxID),
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

func imapMailboxFromFolder(folder Folder, state IMAPUIDState) imapgw.Mailbox {
	return imapgw.Mailbox{
		ID:            imapgw.MailboxID(folder.ID),
		ParentID:      imapgw.MailboxID(folder.ParentID),
		Name:          folder.Name,
		FullPath:      folder.FullPath,
		SystemType:    folder.SystemType,
		UIDValidity:   state.UIDValidity,
		UIDNext:       state.UIDNext,
		HighestModSeq: state.HighestModSeq,
		Messages:      uint32(folder.Total),
		Unseen:        uint32(folder.Unread),
	}
}

type imapMessageRow struct {
	ID           string
	MailboxID    string
	RFCMessageID string
	Subject      string
	FromAddr     string
	FromName     string
	InternalDate time.Time
	Size         int64
	Read         bool
	Starred      bool
	Answered     bool
	Forwarded    bool
	Draft        bool
	Deleted      bool
	Status       string
}

type imapStoreFlagChanges struct {
	Read     *bool
	Starred  *bool
	Answered *bool
	Deleted  *bool
}

func newIMAPStoreFlagChanges(flags imapgw.MessageFlags, mode imapgw.StoreFlagsMode) (imapStoreFlagChanges, error) {
	if flags.Forwarded || flags.Draft || strings.TrimSpace(flags.Status) != "" {
		return imapStoreFlagChanges{}, fmt.Errorf("unsupported imap store flag set")
	}
	var changes imapStoreFlagChanges
	switch mode {
	case imapgw.StoreFlagsAdd:
		if flags.Read {
			changes.Read = boolPointer(true)
		}
		if flags.Starred {
			changes.Starred = boolPointer(true)
		}
		if flags.Answered {
			changes.Answered = boolPointer(true)
		}
		if flags.Deleted {
			changes.Deleted = boolPointer(true)
		}
	case imapgw.StoreFlagsRemove:
		if flags.Read {
			changes.Read = boolPointer(false)
		}
		if flags.Starred {
			changes.Starred = boolPointer(false)
		}
		if flags.Answered {
			changes.Answered = boolPointer(false)
		}
		if flags.Deleted {
			changes.Deleted = boolPointer(false)
		}
	case imapgw.StoreFlagsReplace:
		changes.Read = boolPointer(flags.Read)
		changes.Starred = boolPointer(flags.Starred)
		changes.Answered = boolPointer(flags.Answered)
		changes.Deleted = boolPointer(flags.Deleted)
	default:
		return imapStoreFlagChanges{}, fmt.Errorf("unsupported imap store flags mode %q", mode)
	}
	if changes.Read == nil && changes.Starred == nil && changes.Answered == nil && changes.Deleted == nil {
		return imapStoreFlagChanges{}, fmt.Errorf("imap flags are required")
	}
	return changes, nil
}

func applyIMAPStoreFlagChanges(row imapMessageRow, changes imapStoreFlagChanges) (imapMessageRow, bool) {
	next := row
	if changes.Read != nil {
		next.Read = *changes.Read
	}
	if changes.Starred != nil {
		next.Starred = *changes.Starred
	}
	if changes.Answered != nil {
		next.Answered = *changes.Answered
	}
	if changes.Deleted != nil {
		next.Deleted = *changes.Deleted
	}
	return next, next.Read != row.Read || next.Starred != row.Starred || next.Answered != row.Answered || next.Deleted != row.Deleted
}

func boolPointer(value bool) *bool {
	return &value
}

func normalizeIMAPUIDBackfillLimit(limit int) int {
	if limit <= 0 {
		return imapUIDBackfillDefaultLimit
	}
	if limit > imapUIDBackfillMaxLimit {
		return imapUIDBackfillMaxLimit
	}
	return limit
}

func imapMessageFromRow(row imapMessageRow, uid IMAPMessageUID) imapgw.MessageSummary {
	return imapgw.MessageSummary{
		ID:        imapgw.MessageID(row.ID),
		MailboxID: imapgw.MailboxID(row.MailboxID),
		UID:       uid.UID,
		Envelope: imapgw.Envelope{
			MessageID: row.RFCMessageID,
			Subject:   row.Subject,
			From:      imapEnvelopeAddress(row.FromName, row.FromAddr),
			Date:      row.InternalDate,
		},
		Flags: imapgw.MessageFlags{
			Read:      row.Read,
			Starred:   row.Starred,
			Answered:  row.Answered,
			Forwarded: row.Forwarded,
			Draft:     row.Draft,
			Deleted:   row.Deleted,
			Status:    row.Status,
		},
		InternalDate: row.InternalDate,
		Size:         row.Size,
		ModSeq:       uid.ModSeq,
	}
}

func imapSequenceNumberForUID(ctx context.Context, querier imapSequenceQuerier, userID string, mailboxID string, uid imapgw.UID) (uint32, error) {
	const query = `
SELECT COUNT(*)
FROM imap_message_uid i
JOIN messages m ON m.id = i.message_id
WHERE i.user_id = $1::uuid
  AND i.mailbox_id = $2::uuid
  AND i.uid <= $3
  AND m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'`

	var count int64
	if err := querier.QueryRowContext(ctx, query, userID, mailboxID, int64(uid)).Scan(&count); err != nil {
		return 0, fmt.Errorf("get imap sequence number: %w", err)
	}
	if count <= 0 || count > math.MaxUint32 {
		return 0, fmt.Errorf("imap sequence number unavailable for uid %d", uid)
	}
	return uint32(count), nil
}

func imapEnvelopeAddress(name string, address string) []imapgw.Address {
	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)
	if address == "" {
		return nil
	}
	if parsed, err := mail.ParseAddress(address); err == nil {
		address = parsed.Address
		if name == "" {
			name = parsed.Name
		}
	}
	mailbox, host, ok := strings.Cut(address, "@")
	if !ok {
		return []imapgw.Address{{Name: name, Mailbox: address}}
	}
	return []imapgw.Address{{Name: name, Mailbox: mailbox, Host: host}}
}

func scanIMAPMessageByUID(ctx context.Context, tx *sql.Tx, userID string, mailboxID string, uid imapgw.UID) (imapMessageRow, IMAPMessageUID, error) {
	const query = `
SELECT
  m.id::text,
  m.folder_id::text,
  COALESCE(m.rfc_message_id, ''),
  m.subject,
  m.from_addr,
  m.from_name,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date,
  m.size,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred,
  COALESCE((m.flags->>'answered')::boolean, false) AS answered,
  COALESCE((m.flags->>'forwarded')::boolean, false) AS forwarded,
  COALESCE((m.flags->>'draft')::boolean, false) AS draft,
  COALESCE((m.flags->>'imap_deleted')::boolean, false) AS deleted,
  m.status,
  i.uid,
  i.modseq
FROM imap_message_uid i
JOIN messages m ON m.id = i.message_id
WHERE i.user_id = $1::uuid
  AND i.mailbox_id = $2::uuid
  AND i.uid = $3
  AND m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'
LIMIT 1
FOR UPDATE OF i, m`

	var row imapMessageRow
	var messageUID IMAPMessageUID
	if err := tx.QueryRowContext(ctx, query, userID, mailboxID, int64(uid)).Scan(
		&row.ID,
		&row.MailboxID,
		&row.RFCMessageID,
		&row.Subject,
		&row.FromAddr,
		&row.FromName,
		&row.InternalDate,
		&row.Size,
		&row.Read,
		&row.Starred,
		&row.Answered,
		&row.Forwarded,
		&row.Draft,
		&row.Deleted,
		&row.Status,
		&messageUID.UID,
		&messageUID.ModSeq,
	); err != nil {
		if err == sql.ErrNoRows {
			return imapMessageRow{}, IMAPMessageUID{}, fmt.Errorf("imap message uid %d not found", uid)
		}
		return imapMessageRow{}, IMAPMessageUID{}, fmt.Errorf("scan imap message by uid: %w", err)
	}
	messageUID.MessageID = imapgw.MessageID(row.ID)
	messageUID.MailboxID = imapgw.MailboxID(row.MailboxID)
	if err := imapgw.ValidateMessageUID(messageUID); err != nil {
		return imapMessageRow{}, IMAPMessageUID{}, err
	}
	return row, messageUID, nil
}

func updateIMAPMessageFlags(ctx context.Context, tx *sql.Tx, messageID string, modseq uint64, row imapMessageRow) error {
	const updateMessage = `
UPDATE messages
SET flags = jsonb_set(
      jsonb_set(
        jsonb_set(
          jsonb_set(flags, '{read}', to_jsonb($2::boolean), true),
          '{starred}', to_jsonb($3::boolean), true
        ),
        '{answered}', to_jsonb($4::boolean), true
      ),
      '{imap_deleted}', to_jsonb($5::boolean), true
    ),
    updated_at = now()
WHERE id = $1::uuid`
	if _, err := tx.ExecContext(ctx, updateMessage, messageID, row.Read, row.Starred, row.Answered, row.Deleted); err != nil {
		return fmt.Errorf("update imap message flags: %w", err)
	}

	const updateUID = `
UPDATE imap_message_uid
SET modseq = $2,
    updated_at = now()
WHERE message_id = $1::uuid`
	if _, err := tx.ExecContext(ctx, updateUID, messageID, int64(modseq)); err != nil {
		return fmt.Errorf("update imap message modseq: %w", err)
	}
	return nil
}

func updateIMAPMailboxModSeq(ctx context.Context, tx *sql.Tx, mailboxID string, highestModSeq uint64) error {
	const query = `
UPDATE imap_mailbox_state
SET highest_modseq = $2,
    updated_at = now()
WHERE mailbox_id = $1::uuid`
	if _, err := tx.ExecContext(ctx, query, mailboxID, int64(highestModSeq)); err != nil {
		return fmt.Errorf("update imap mailbox modseq: %w", err)
	}
	return nil
}

func (r *Repository) EnsureIMAPMailboxState(ctx context.Context, userID string, mailboxID string) (IMAPUIDState, error) {
	if r.db == nil {
		return IMAPUIDState{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	if userID == "" {
		return IMAPUIDState{}, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
		return IMAPUIDState{}, fmt.Errorf("mailbox_id is required")
	}

	const query = `
INSERT INTO imap_mailbox_state (mailbox_id, user_id)
SELECT id, user_id
FROM folders
WHERE id = $1::uuid
  AND user_id = $2::uuid
ON CONFLICT (mailbox_id) DO UPDATE
SET updated_at = imap_mailbox_state.updated_at
RETURNING mailbox_id::text, uidvalidity, uidnext, highest_modseq`

	var state IMAPUIDState
	if err := r.db.QueryRowContext(ctx, query, mailboxID, userID).Scan(
		&state.MailboxID,
		&state.UIDValidity,
		&state.UIDNext,
		&state.HighestModSeq,
	); err != nil {
		return IMAPUIDState{}, fmt.Errorf("ensure imap mailbox state: %w", err)
	}
	if err := imapgw.ValidateUIDState(state); err != nil {
		return IMAPUIDState{}, err
	}
	return state, nil
}

func (r *Repository) EnsureIMAPMessageUID(ctx context.Context, userID string, mailboxID string, messageID string) (IMAPMessageUID, error) {
	if r.db == nil {
		return IMAPMessageUID{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	mailboxID = strings.TrimSpace(mailboxID)
	messageID = strings.TrimSpace(messageID)
	if userID == "" {
		return IMAPMessageUID{}, fmt.Errorf("user_id is required")
	}
	if mailboxID == "" {
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
  SELECT id, folder_id, user_id
  FROM messages
  WHERE id = $1::uuid
    AND folder_id = $2::uuid
    AND user_id = $3::uuid
    AND status = 'active'
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
		if err == sql.ErrNoRows {
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
