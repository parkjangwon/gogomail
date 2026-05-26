package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/lib/pq"
)

func (r *Repository) CopyIMAPMessages(ctx context.Context, userID string, sourceMailboxID string, destMailboxID string, uids []imapgw.UID) ([]imapgw.CopyMessageResult, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(sourceMailboxID) == "" {
		return nil, fmt.Errorf("source_mailbox_id is required")
	}
	if strings.TrimSpace(destMailboxID) == "" {
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
	uidsArray := imapUIDArray(uids)

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
  FROM unnest($4::bigint[]) AS input(value)
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
SELECT COALESCE(SUM(copied_size), 0), COUNT(*)
FROM source_bytes
WHERE EXISTS (
  SELECT 1
  FROM folders f
  WHERE f.id = $3::uuid
    AND f.user_id = $1::uuid
)`
	var sourceCount int64
	if err := tx.QueryRowContext(ctx, totalQuery, userID, sourceMailboxID, destMailboxID, pq.Array(uidsArray)).Scan(&totalSize, &sourceCount); err != nil {
		return nil, fmt.Errorf("sum imap copy message sizes: %w", err)
	}
	if err := checkAndIncrementUserQuota(ctx, tx, userID, totalSize); err != nil {
		return nil, err
	}
	if err := ensureIMAPUIDAllocationCapacity(ctx, tx, userID, destMailboxID, sourceCount); err != nil {
		return nil, err
	}

	const copyQuery = `
WITH input AS (
  SELECT value::bigint AS uid, ordinality
  FROM unnest($4::bigint[]) WITH ORDINALITY AS input(value, ordinality)
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
    i.uid AS source_uid,
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
locked_unassigned AS (
  SELECT
    m.id,
    COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date
  FROM messages m
  LEFT JOIN imap_message_uid i
    ON i.message_id = m.id
   AND i.user_id = m.user_id
   AND i.mailbox_id = m.folder_id
  WHERE m.user_id = $1::uuid
    AND m.folder_id = $3::uuid
    AND m.status = 'active'
    AND i.message_id IS NULL
  FOR UPDATE OF m
),
unassigned_existing AS (
  SELECT
    id,
    row_number() OVER (ORDER BY internal_date, id) AS rn
  FROM locked_unassigned
),
backfilled_existing AS (
  INSERT INTO imap_message_uid (message_id, mailbox_id, user_id, uid, modseq)
  SELECT
    unassigned_existing.id,
    locked_state.mailbox_id,
    locked_state.user_id,
    locked_state.uidnext + unassigned_existing.rn - 1,
    locked_state.highest_modseq + unassigned_existing.rn
  FROM unassigned_existing
  CROSS JOIN locked_state
  WHERE EXISTS (SELECT 1 FROM source)
  ON CONFLICT (message_id) DO NOTHING
  RETURNING 1
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
    locked_state.uidnext + (SELECT COUNT(*) FROM backfilled_existing) + source.rn - 1,
    locked_state.highest_modseq + (SELECT COUNT(*) FROM backfilled_existing) + source.rn
  FROM source
  CROSS JOIN locked_state
  RETURNING message_id, mailbox_id, user_id, uid, modseq
),
updated_state AS (
  UPDATE imap_mailbox_state
  SET uidnext = uidnext + (SELECT COUNT(*) FROM backfilled_existing) + (SELECT COUNT(*) FROM source),
      highest_modseq = highest_modseq + (SELECT COUNT(*) FROM backfilled_existing) + (SELECT COUNT(*) FROM source),
      updated_at = CASE WHEN EXISTS (SELECT 1 FROM backfilled_existing) OR EXISTS (SELECT 1 FROM source) THEN now() ELSE updated_at END
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
  source.to_addrs,
  source.cc_addrs,
  source.bcc_addrs,
  COALESCE(source.received_at, source.sent_at, now()) AS internal_date,
  source.size,
  COALESCE((source.flags->>'read')::boolean, false) AS read,
  COALESCE((source.flags->>'starred')::boolean, false) AS starred,
  COALESCE((source.flags->>'answered')::boolean, false) AS answered,
  COALESCE((source.flags->>'forwarded')::boolean, false) AS forwarded,
  COALESCE((source.flags->>'draft')::boolean, false) AS draft,
  COALESCE((source.flags->>'imap_deleted')::boolean, false) AS deleted,
  CASE
    WHEN jsonb_typeof(source.flags->'imap_keywords') = 'array' THEN source.flags->'imap_keywords'
    ELSE '[]'::jsonb
  END AS imap_keywords,
  'active' AS status,
  source.source_uid,
  inserted_uids.uid,
  inserted_uids.modseq,
  (SELECT COUNT(*) FROM inserted_attachments) AS attachment_copy_count
FROM source
JOIN inserted_messages ON inserted_messages.id = source.new_id
JOIN inserted_uids ON inserted_uids.message_id = source.new_id
ORDER BY source.rn`
	rows, err := tx.QueryContext(ctx, copyQuery, userID, sourceMailboxID, destMailboxID, pq.Array(uidsArray))
	if err != nil {
		return nil, fmt.Errorf("copy imap messages: %w", err)
	}
	defer rows.Close()

	results := make([]imapgw.CopyMessageResult, 0, len(uids))
	for rows.Next() {
		var row imapMessageRow
		var messageUID IMAPMessageUID
		var sourceUID imapgw.UID
		var attachmentCopyCount int64
		if err := rows.Scan(
			&row.ID,
			&row.MailboxID,
			&row.RFCMessageID,
			&row.Subject,
			&row.FromAddr,
			&row.FromName,
			&row.ToAddrs,
			&row.CcAddrs,
			&row.BccAddrs,
			&row.InternalDate,
			&row.Size,
			&row.Read,
			&row.Starred,
			&row.Answered,
			&row.Forwarded,
			&row.Draft,
			&row.Deleted,
			&row.Keywords,
			&row.Status,
			&sourceUID,
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
		results = append(results, imapgw.CopyMessageResult{SourceUID: sourceUID, Destination: summary})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate copied imap messages: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit imap copy transaction: %w", err)
	}
	return results, nil
}

func (r *Repository) ExpungeIMAPMessages(ctx context.Context, userID string, mailboxID string, uids []imapgw.UID) ([]imapgw.MessageSummary, error) {
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
	if len(uids) > 500 {
		return nil, fmt.Errorf("too many uids")
	}
	for _, uid := range uids {
		if uid == 0 {
			return nil, fmt.Errorf("uid must not be zero")
		}
	}
	uidsArray := imapUIDArray(uids)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin imap expunge transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
WITH input AS (
  SELECT value::bigint AS uid
  FROM unnest($4::bigint[]) AS input(value)
)
SELECT
  m.id::text,
  m.folder_id::text,
  COALESCE(m.rfc_message_id, ''),
  m.subject,
  m.from_addr,
  m.from_name,
  m.to_addrs,
  m.cc_addrs,
  m.bcc_addrs,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date,
  m.size,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred,
  COALESCE((m.flags->>'answered')::boolean, false) AS answered,
  COALESCE((m.flags->>'forwarded')::boolean, false) AS forwarded,
  COALESCE((m.flags->>'draft')::boolean, false) AS draft,
  COALESCE((m.flags->>'imap_deleted')::boolean, false) AS deleted,
  CASE
    WHEN jsonb_typeof(m.flags->'imap_keywords') = 'array' THEN m.flags->'imap_keywords'
    ELSE '[]'::jsonb
  END AS imap_keywords,
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
	rows, err := tx.QueryContext(ctx, query, userID, mailboxID, len(uids) > 0, pq.Array(uidsArray))
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
			&row.ToAddrs,
			&row.CcAddrs,
			&row.BccAddrs,
			&row.InternalDate,
			&row.Size,
			&row.Read,
			&row.Starred,
			&row.Answered,
			&row.Forwarded,
			&row.Draft,
			&row.Deleted,
			&row.Keywords,
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
	if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET status = 'deleted',
    deleted_at = now(),
    updated_at = now()
WHERE user_id = $1::uuid
  AND id IN (SELECT value::uuid FROM unnest($2::uuid[]) AS input(value))
  AND status = 'active'`, userID, pq.Array(messageIDs)); err != nil {
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

// LookupExpungeStoragePaths returns unique non-empty storage paths for IMAP
// messages that are flagged as \Deleted in the given mailbox and would be
// removed by an EXPUNGE command.  Only paths not shared with any other message
// record are returned, so the caller can safely delete those objects from the
// backing store after the database records are removed.  Pass a nil or empty
// uids slice to match all \Deleted messages in the mailbox.
func (r *Repository) LookupExpungeStoragePaths(ctx context.Context, userID string, mailboxID string, uids []imapgw.UID) ([]string, error) {
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
	uidsArray := imapUIDArray(uids)
	rows, err := r.db.QueryContext(ctx, lookupExpungeStoragePathsSQL, userID, mailboxID, len(uids) > 0, pq.Array(uidsArray))
	if err != nil {
		return nil, fmt.Errorf("lookup expunge storage paths: %w", err)
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan expunge storage path: %w", err)
		}
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expunge storage paths: %w", err)
	}
	return paths, nil
}

const lookupExpungeStoragePathsSQL = `
WITH target AS (
  SELECT DISTINCT m.storage_path
  FROM imap_message_uid i
  JOIN messages m ON m.id = i.message_id
  WHERE i.user_id = $1::uuid
    AND i.mailbox_id = $2::uuid
    AND m.user_id = $1::uuid
    AND m.folder_id = $2::uuid
    AND m.status = 'active'
    AND COALESCE((m.flags->>'imap_deleted')::boolean, false) = true
    AND ($3::bool = false OR i.uid IN (SELECT value::bigint FROM unnest($4::bigint[]) AS input(value)))
    AND m.storage_path IS NOT NULL
    AND m.storage_path <> ''
),
ref_counts AS (
  SELECT ref.storage_path, COUNT(*) AS ref_count
  FROM messages ref
  JOIN target ON target.storage_path = ref.storage_path
  GROUP BY ref.storage_path
)
SELECT target.storage_path
FROM target
JOIN ref_counts ON ref_counts.storage_path = target.storage_path
WHERE ref_counts.ref_count = 1`

func (r *Repository) MoveIMAPMessages(ctx context.Context, userID string, sourceMailboxID string, destMailboxID string, uids []imapgw.UID) ([]imapgw.MoveMessageResult, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(sourceMailboxID) == "" {
		return nil, fmt.Errorf("source_mailbox_id is required")
	}
	if strings.TrimSpace(destMailboxID) == "" {
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
	uidsArray := imapUIDArray(uids)

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
  FROM unnest($3::bigint[]) WITH ORDINALITY AS input(value, ordinality)
)
SELECT
  m.id::text,
  m.folder_id::text,
  COALESCE(m.rfc_message_id, ''),
  m.subject,
  m.from_addr,
  m.from_name,
  m.to_addrs,
  m.cc_addrs,
  m.bcc_addrs,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date,
  m.size,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred,
  COALESCE((m.flags->>'answered')::boolean, false) AS answered,
  COALESCE((m.flags->>'forwarded')::boolean, false) AS forwarded,
  COALESCE((m.flags->>'draft')::boolean, false) AS draft,
  COALESCE((m.flags->>'imap_deleted')::boolean, false) AS deleted,
  CASE
    WHEN jsonb_typeof(m.flags->'imap_keywords') = 'array' THEN m.flags->'imap_keywords'
    ELSE '[]'::jsonb
  END AS imap_keywords,
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
	rows, err := tx.QueryContext(ctx, query, userID, sourceMailboxID, pq.Array(uidsArray))
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
			&row.ToAddrs,
			&row.CcAddrs,
			&row.BccAddrs,
			&row.InternalDate,
			&row.Size,
			&row.Read,
			&row.Starred,
			&row.Answered,
			&row.Forwarded,
			&row.Draft,
			&row.Deleted,
			&row.Keywords,
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
	backfilledDest, err := backfillIMAPMailboxUIDsTx(ctx, tx, userID, destMailboxID, destState.uidNext, destState.highestModSeq)
	if err != nil {
		return nil, err
	}
	destState.uidNext += imapgw.UID(backfilledDest)
	destState.highestModSeq += uint64(backfilledDest)
	if uint64(destState.uidNext)+uint64(len(messageIDs)) > math.MaxUint32 {
		return nil, fmt.Errorf("imap destination uidnext exhausted")
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET folder_id = $3::uuid,
    updated_at = now()
WHERE user_id = $1::uuid
  AND id IN (SELECT value::uuid FROM unnest($2::uuid[]) AS input(value))
  AND status = 'active'`, userID, pq.Array(messageIDs), destMailboxID); err != nil {
		return nil, fmt.Errorf("move imap messages: %w", err)
	}

	destUIDValues := make([]int64, len(sourceSummaries))
	destModSeqValues := make([]int64, len(sourceSummaries))
	for i := range sourceSummaries {
		destUIDValues[i] = int64(destState.uidNext + imapgw.UID(i))
		destModSeqValues[i] = int64(destState.highestModSeq + uint64(i) + 1)
	}
	const bulkUpdateUID = `
UPDATE imap_message_uid AS t
SET mailbox_id = $3::uuid,
    uid = u.new_uid,
    modseq = u.new_modseq,
    updated_at = now()
FROM unnest($4::uuid[], $5::bigint[], $6::bigint[])
  WITH ORDINALITY AS u(message_id, new_uid, new_modseq, ord)
WHERE t.message_id = u.message_id
  AND t.user_id = $1::uuid
  AND t.mailbox_id = $2::uuid`
	res, err := tx.ExecContext(ctx, bulkUpdateUID,
		userID,
		sourceMailboxID,
		destMailboxID,
		pq.Array(messageIDs),
		pq.Array(destUIDValues),
		pq.Array(destModSeqValues),
	)
	if err != nil {
		return nil, fmt.Errorf("move imap message uid: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return nil, fmt.Errorf("read moved imap uid count: %w", err)
	} else if affected != int64(len(messageIDs)) {
		return nil, fmt.Errorf("move imap message uid affected %d rows (expected %d)", affected, len(messageIDs))
	}

	// Compute sequence numbers for all moved UIDs in the destination mailbox in
	// a single window-function query.
	destSequenceNumbers, err := imapSequenceNumbersForUIDs(ctx, tx, userID, destMailboxID, destUIDValues)
	if err != nil {
		return nil, err
	}

	results := make([]imapgw.MoveMessageResult, 0, len(sourceSummaries))
	for i, source := range sourceSummaries {
		sourceHighestModSeq := sourceState.highestModSeq + uint64(i) + 1
		destUID := imapgw.UID(destUIDValues[i])
		destModSeq := uint64(destModSeqValues[i])

		destUIDRecord := IMAPMessageUID{
			MessageID: imapgw.MessageID(sourceRows[i].ID),
			MailboxID: imapgw.MailboxID(destMailboxID),
			UID:       destUID,
			ModSeq:    destModSeq,
		}
		destRow := sourceRows[i]
		destRow.MailboxID = destMailboxID
		destination := imapMessageFromRow(destRow, destUIDRecord)
		sequenceNumber, ok := destSequenceNumbers[int64(destUID)]
		if !ok {
			return nil, fmt.Errorf("imap sequence number unavailable for uid %d", destUID)
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

func backfillIMAPMailboxUIDsTx(ctx context.Context, tx *sql.Tx, userID string, mailboxID string, uidNext imapgw.UID, highestModSeq uint64) (int, error) {
	if err := lockIMAPMailboxFolderForUIDAllocation(ctx, tx, userID, mailboxID); err != nil {
		return 0, err
	}

	// Process in bounded batches to avoid long-held locks and OOM on large mailboxes.
	const backfillBatchSize = 1000
	const selectMessages = `
SELECT m.id::text
FROM messages m
LEFT JOIN imap_message_uid i
  ON i.message_id = m.id
 AND i.user_id = m.user_id
 AND i.mailbox_id = m.folder_id
WHERE m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'
  AND i.message_id IS NULL
ORDER BY COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at), m.id
LIMIT $3
FOR UPDATE OF m`
	rows, err := tx.QueryContext(ctx, selectMessages, userID, mailboxID, backfillBatchSize)
	if err != nil {
		return 0, fmt.Errorf("select imap destination uid backfill messages: %w", err)
	}
	var messageIDs []string
	for rows.Next() {
		var messageID string
		if err := rows.Scan(&messageID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan imap destination uid backfill message: %w", err)
		}
		messageIDs = append(messageIDs, messageID)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close imap destination uid backfill rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate imap destination uid backfill messages: %w", err)
	}
	if len(messageIDs) == 0 {
		return 0, nil
	}
	if uint64(uidNext)+uint64(len(messageIDs)) > math.MaxUint32 {
		return 0, fmt.Errorf("imap destination uidnext exhausted")
	}

	const insertUID = `
INSERT INTO imap_message_uid (message_id, mailbox_id, user_id, uid, modseq)
SELECT
  m.id::uuid,
  $2::uuid,
  $3::uuid,
  $4 + (m.ord - 1),
  $5 + m.ord
FROM unnest($1::uuid[]) WITH ORDINALITY AS m(id, ord)`
	if _, err := tx.ExecContext(ctx, insertUID, pq.Array(messageIDs), mailboxID, userID, int64(uidNext), int64(highestModSeq)); err != nil {
		return 0, fmt.Errorf("insert imap destination uid backfill rows: %w", err)
	}
	return len(messageIDs), nil
}

func ensureIMAPUIDAllocationCapacity(ctx context.Context, tx *sql.Tx, userID string, mailboxID string, additional int64) error {
	if additional <= 0 {
		return nil
	}
	var uidNext imapgw.UID
	const lockState = `
SELECT uidnext
FROM imap_mailbox_state
WHERE mailbox_id = $1::uuid
  AND user_id = $2::uuid
FOR UPDATE`
	if err := tx.QueryRowContext(ctx, lockState, mailboxID, userID).Scan(&uidNext); err != nil {
		return fmt.Errorf("lock imap uid allocation state: %w", err)
	}
	if err := lockIMAPMailboxFolderForUIDAllocation(ctx, tx, userID, mailboxID); err != nil {
		return err
	}

	var unassigned int64
	const countUnassigned = `
SELECT COUNT(*)
FROM messages m
LEFT JOIN imap_message_uid i
  ON i.message_id = m.id
 AND i.user_id = m.user_id
 AND i.mailbox_id = m.folder_id
WHERE m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'
  AND i.message_id IS NULL`
	if err := tx.QueryRowContext(ctx, countUnassigned, userID, mailboxID).Scan(&unassigned); err != nil {
		return fmt.Errorf("count imap uid allocation backlog: %w", err)
	}
	if uint64(uidNext)+uint64(unassigned)+uint64(additional) > math.MaxUint32 {
		return fmt.Errorf("imap uid space exhausted")
	}
	return nil
}

func lockIMAPMailboxFolderForUIDAllocation(ctx context.Context, tx *sql.Tx, userID string, mailboxID string) error {
	const query = `
SELECT id
FROM folders
WHERE id = $1::uuid
  AND user_id = $2::uuid
FOR UPDATE`
	var id string
	if err := tx.QueryRowContext(ctx, query, mailboxID, userID).Scan(&id); err != nil {
		return fmt.Errorf("lock imap uid allocation folder: %w", err)
	}
	return nil
}

func countIMAPSourceUIDsTx(ctx context.Context, tx *sql.Tx, userID string, mailboxID string, uidsArray []string) (int64, error) {
	const query = `
WITH input AS (
  SELECT value::bigint AS uid
  FROM unnest($3::bigint[]) AS input(value)
)
SELECT COUNT(*)
FROM input
JOIN imap_message_uid i
  ON i.uid = input.uid
 AND i.user_id = $1::uuid
 AND i.mailbox_id = $2::uuid
JOIN messages m
  ON m.id = i.message_id
 AND m.user_id = $1::uuid
 AND m.folder_id = $2::uuid
 AND m.status = 'active'`
	var count int64
	if err := tx.QueryRowContext(ctx, query, userID, mailboxID, pq.Array(uidsArray)).Scan(&count); err != nil {
		return 0, fmt.Errorf("count imap source uids: %w", err)
	}
	return count, nil
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
	uidsArray := imapUIDArray(uids)

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
	moveCount, err := countIMAPSourceUIDsTx(ctx, tx, userID, mailboxID, uidsArray)
	if err != nil {
		return nil, err
	}
	if err := ensureIMAPUIDAllocationCapacity(ctx, tx, userID, mailboxID, moveCount); err != nil {
		return nil, err
	}

	const query = `
WITH input AS (
  SELECT value::bigint AS uid, ordinality
  FROM unnest($3::bigint[]) WITH ORDINALITY AS input(value, ordinality)
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
locked_unassigned AS (
  SELECT
    m.id,
    COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date
  FROM messages m
  LEFT JOIN imap_message_uid i
    ON i.message_id = m.id
   AND i.user_id = m.user_id
   AND i.mailbox_id = m.folder_id
  WHERE m.user_id = $1::uuid
    AND m.folder_id = $2::uuid
    AND m.status = 'active'
    AND i.message_id IS NULL
  FOR UPDATE OF m
),
unassigned_existing AS (
  SELECT
    id,
    row_number() OVER (ORDER BY internal_date, id) AS rn
  FROM locked_unassigned
),
backfilled_existing AS (
  INSERT INTO imap_message_uid (message_id, mailbox_id, user_id, uid, modseq)
  SELECT
    unassigned_existing.id,
    locked_state.mailbox_id,
    locked_state.user_id,
    locked_state.uidnext + unassigned_existing.rn - 1,
    locked_state.highest_modseq + unassigned_existing.rn
  FROM unassigned_existing
  CROSS JOIN locked_state
  WHERE EXISTS (SELECT 1 FROM source)
  ON CONFLICT (message_id) DO NOTHING
  RETURNING 1
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
    locked_state.uidnext + (SELECT COUNT(*) FROM backfilled_existing) + source.rn - 1,
    locked_state.highest_modseq + (SELECT COUNT(*) FROM backfilled_existing) + source.rn
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
  SET uidnext = uidnext + (SELECT COUNT(*) FROM backfilled_existing) + (SELECT COUNT(*) FROM source),
      highest_modseq = highest_modseq + (SELECT COUNT(*) FROM backfilled_existing) + (SELECT COUNT(*) FROM source),
      updated_at = CASE WHEN EXISTS (SELECT 1 FROM backfilled_existing) OR EXISTS (SELECT 1 FROM source) THEN now() ELSE updated_at END
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
  source.to_addrs,
  source.cc_addrs,
  source.bcc_addrs,
  COALESCE(source.received_at, source.sent_at, source.draft_updated_at, source.created_at) AS source_internal_date,
  source.size,
  COALESCE((source.flags->>'read')::boolean, false) AS source_read,
  COALESCE((source.flags->>'starred')::boolean, false) AS source_starred,
  COALESCE((source.flags->>'answered')::boolean, false) AS source_answered,
  COALESCE((source.flags->>'forwarded')::boolean, false) AS source_forwarded,
  COALESCE((source.flags->>'draft')::boolean, false) AS source_draft,
  COALESCE((source.flags->>'imap_deleted')::boolean, false) AS source_deleted,
  CASE
    WHEN jsonb_typeof(source.flags->'imap_keywords') = 'array' THEN source.flags->'imap_keywords'
    ELSE '[]'::jsonb
  END AS source_imap_keywords,
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
	rows, err := tx.QueryContext(ctx, query, userID, mailboxID, pq.Array(uidsArray))
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
			&sourceRow.ToAddrs,
			&sourceRow.CcAddrs,
			&sourceRow.BccAddrs,
			&sourceRow.InternalDate,
			&sourceRow.Size,
			&sourceRow.Read,
			&sourceRow.Starred,
			&sourceRow.Answered,
			&sourceRow.Forwarded,
			&sourceRow.Draft,
			&sourceRow.Deleted,
			&sourceRow.Keywords,
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
