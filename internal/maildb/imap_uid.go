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
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/lib/pq"
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
	if len(folders) == 0 {
		return nil, nil
	}

	states, err := r.ensureIMAPMailboxStates(ctx, userID, folders)
	if err != nil {
		return nil, err
	}

	mailboxes := make([]imapgw.Mailbox, 0, len(folders))
	for _, folder := range folders {
		state, ok := states[folder.ID]
		if !ok {
			return nil, fmt.Errorf("imap mailbox state missing for folder %q", folder.ID)
		}
		mailboxes = append(mailboxes, imapMailboxFromFolder(folder, state))
	}
	return mailboxes, nil
}

// ensureIMAPMailboxStates upserts imap_mailbox_state rows for all given folders
// in a single query and returns a map of folderID → IMAPUIDState.
func (r *Repository) ensureIMAPMailboxStates(ctx context.Context, userID string, folders []Folder) (map[string]IMAPUIDState, error) {
	folderIDs := make([]string, len(folders))
	for i, f := range folders {
		folderIDs[i] = f.ID
	}

	const query = `
INSERT INTO imap_mailbox_state (mailbox_id, user_id)
SELECT id, user_id
FROM folders
WHERE user_id = $1::uuid
  AND id = ANY($2::uuid[])
ON CONFLICT (mailbox_id) DO UPDATE
SET updated_at = imap_mailbox_state.updated_at
RETURNING mailbox_id::text, uidvalidity, uidnext, highest_modseq`

	rows, err := r.db.QueryContext(ctx, query, userID, pq.Array(folderIDs))
	if err != nil {
		return nil, fmt.Errorf("ensure imap mailbox states: %w", err)
	}
	defer rows.Close()

	states := make(map[string]IMAPUIDState, len(folders))
	for rows.Next() {
		var state IMAPUIDState
		if err := rows.Scan(&state.MailboxID, &state.UIDValidity, &state.UIDNext, &state.HighestModSeq); err != nil {
			return nil, fmt.Errorf("scan imap mailbox state: %w", err)
		}
		if err := imapgw.ValidateUIDState(state); err != nil {
			return nil, err
		}
		states[string(state.MailboxID)] = state
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate imap mailbox states: %w", err)
	}
	return states, nil
}

func (r *Repository) GetIMAPMailbox(ctx context.Context, userID string, mailboxID string) (imapgw.Mailbox, error) {
	if r.db == nil {
		return imapgw.Mailbox{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return imapgw.Mailbox{}, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(mailboxID) == "" {
		return imapgw.Mailbox{}, fmt.Errorf("mailbox_id is required")
	}
	exactMailboxName := strings.ToLower(mailboxID)
	compatMailboxName, allowCompatMailboxLookup := normalizeIMAPMailboxLookupName(mailboxID)
	var mailboxIDUUID any
	if isUUIDLike(mailboxID) {
		mailboxIDUUID = strings.TrimSpace(mailboxID)
	}

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
  COALESCE(c.starred, 0) AS starred,
  COALESCE(c.total_size, 0) AS total_size,
  COALESCE(c.imap_unassigned, 0) AS imap_unassigned
FROM folders f
LEFT JOIN (
  SELECT
    m.folder_id,
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE COALESCE((flags->>'read')::boolean, false) = false) AS unread,
    COUNT(*) FILTER (WHERE COALESCE((flags->>'starred')::boolean, false) = true) AS starred,
    SUM(size) AS total_size,
    COUNT(*) FILTER (WHERE i.message_id IS NULL) AS imap_unassigned
  FROM messages m
  LEFT JOIN imap_message_uid i
    ON i.message_id = m.id
   AND i.user_id = m.user_id
   AND i.mailbox_id = m.folder_id
  WHERE m.user_id = $1::uuid
    AND m.status = 'active'
  GROUP BY m.folder_id
) c ON c.folder_id = f.id
WHERE f.user_id = $1::uuid
  AND (
    ($2::uuid IS NOT NULL AND f.id = $2::uuid)
    OR lower(f.name) = $3
    OR (
      $5
      AND (
        lower(f.name) = $4
        OR lower(trim(both '/' from f.full_path)) = $4
        OR ($4 = 'inbox' AND lower(COALESCE(f.system_type, '')) = 'inbox')
      )
    )
  )
ORDER BY
  CASE WHEN lower(COALESCE(f.system_type, '')) = 'inbox' THEN 0 ELSE 1 END,
  f.full_path,
  f.name
LIMIT 1`

	var folder Folder
	if err := r.db.QueryRowContext(ctx, query, userID, mailboxIDUUID, exactMailboxName, compatMailboxName, allowCompatMailboxLookup).Scan(
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
		&folder.TotalSize,
		&folder.IMAPUnassigned,
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

func normalizeIMAPMailboxLookupName(value string) (string, bool) {
	allowCompatLookup := value == strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	value = strings.Trim(value, "/")
	value = collapseIMAPMailboxLookupWhitespace(value)
	return strings.ToLower(value), allowCompatLookup
}

func collapseIMAPMailboxLookupWhitespace(value string) string {
	if value == "" {
		return ""
	}
	out := make([]byte, 0, len(value))
	inSpace := false
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case ' ', '\t', '\r', '\n':
			if len(out) > 0 {
				inSpace = true
			}
		default:
			if inSpace {
				out = append(out, ' ')
				inSpace = false
			}
			out = append(out, value[i])
		}
	}
	return string(out)
}

func (r *Repository) ListIMAPMessages(ctx context.Context, userID string, mailboxID string, limit int, afterUID imapgw.UID) ([]imapgw.MessageSummary, error) {
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
	limit = NormalizeMessageListLimit(limit)

	if _, err := r.EnsureIMAPMailboxState(ctx, userID, mailboxID); err != nil {
		return nil, err
	}

	query := buildListIMAPMessagesQuery(afterUID)

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
	if len(messages) > 0 {
		baseSequence, err := imapSequenceBaseForAfterUID(ctx, r.db, userID, mailboxID, afterUID)
		if err != nil {
			return nil, err
		}
		if err := assignIMAPListSequenceNumbers(messages, baseSequence); err != nil {
			return nil, err
		}
	}
	return messages, nil
}

func buildListIMAPMessagesQuery(afterUID imapgw.UID) string {
	if afterUID > 0 {
		return listIMAPMessagesAfterUIDQuery
	}
	return listIMAPMessagesInitialQuery
}

const listIMAPMessagesSelectColumns = `
SELECT
  m.id::text AS id,
  m.folder_id::text AS folder_id,
  COALESCE(m.rfc_message_id, '') AS rfc_message_id,
  m.subject AS subject,
  m.from_addr AS from_addr,
  m.from_name AS from_name,
  m.to_addrs AS to_addrs,
  m.cc_addrs AS cc_addrs,
  m.bcc_addrs AS bcc_addrs,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS internal_date,
  m.size AS size,
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
  m.status AS status,
  i.uid AS uid,
  i.modseq AS modseq`

const listIMAPMessagesOuterColumns = `
SELECT
  id,
  folder_id,
  rfc_message_id,
  subject,
  from_addr,
  from_name,
  to_addrs,
  cc_addrs,
  bcc_addrs,
  internal_date,
  size,
  read,
  starred,
  answered,
  forwarded,
  draft,
  deleted,
  imap_keywords,
  status,
  uid,
  modseq`

const listIMAPMessagesInitialQuery = listIMAPMessagesSelectColumns + `
FROM messages m
LEFT JOIN imap_message_uid i ON i.message_id = m.id
WHERE m.user_id = $1::uuid
  AND m.folder_id = $2::uuid
  AND m.status = 'active'
ORDER BY
  CASE WHEN i.uid IS NULL THEN 1 ELSE 0 END,
  i.uid,
  internal_date,
  m.id
LIMIT $4`

const listIMAPMessagesAfterUIDQuery = listIMAPMessagesOuterColumns + `
FROM (` + listIMAPMessagesSelectColumns + `
  FROM messages m
  JOIN imap_message_uid i ON i.message_id = m.id
  WHERE m.user_id = $1::uuid
    AND m.folder_id = $2::uuid
    AND m.status = 'active'
    AND i.uid > $3
  UNION ALL` + listIMAPMessagesSelectColumns + `
  FROM messages m
  LEFT JOIN imap_message_uid i ON i.message_id = m.id
  WHERE m.user_id = $1::uuid
    AND m.folder_id = $2::uuid
    AND m.status = 'active'
    AND i.message_id IS NULL
) imap_candidates
ORDER BY
  CASE WHEN uid IS NULL THEN 1 ELSE 0 END,
  uid,
  internal_date,
  id
LIMIT $4`

func (r *Repository) GetIMAPMessage(ctx context.Context, userID string, mailboxID string, uid imapgw.UID) (IMAPStoredMessage, error) {
	if r.db == nil {
		return IMAPStoredMessage{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return IMAPStoredMessage{}, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(mailboxID) == "" {
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

func (r *Repository) StoreIMAPFlags(ctx context.Context, userID string, mailboxID string, uids []imapgw.UID, flags imapgw.MessageFlags, mode imapgw.StoreFlagsMode, unchangedSince uint64, unchangedSinceSet bool) ([]imapgw.MessageSummary, error) {
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
		if unchangedSinceSet && messageUID.ModSeq > unchangedSince {
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

func imapUIDArray(uids []imapgw.UID) []string {
	if len(uids) == 0 {
		return nil
	}
	values := make([]string, 0, len(uids))
	for _, uid := range uids {
		values = append(values, strconv.FormatUint(uint64(uid), 10))
	}
	return values
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

func imapMailboxFromFolder(folder Folder, state IMAPUIDState) imapgw.Mailbox {
	uidNext := state.UIDNext
	highestModSeq := state.HighestModSeq
	if folder.IMAPUnassigned > 0 {
		uidNext = addIMAPUIDOffset(uidNext, folder.IMAPUnassigned)
		highestModSeq = addIMAPModSeqOffset(highestModSeq, folder.IMAPUnassigned)
	}
	return imapgw.Mailbox{
		ID:            imapgw.MailboxID(folder.ID),
		ParentID:      imapgw.MailboxID(folder.ParentID),
		Name:          folder.Name,
		FullPath:      folder.FullPath,
		SystemType:    folder.SystemType,
		UIDValidity:   state.UIDValidity,
		UIDNext:       uidNext,
		HighestModSeq: highestModSeq,
		Messages:      uint32(folder.Total),
		Unseen:        uint32(folder.Unread),
		Size:          folder.TotalSize,
	}
}

func addIMAPUIDOffset(uid imapgw.UID, offset int64) imapgw.UID {
	if offset <= 0 {
		return uid
	}
	if uint64(uid)+uint64(offset) > uint64(^uint32(0)) {
		return imapgw.UID(^uint32(0))
	}
	return imapgw.UID(uint32(uid) + uint32(offset))
}

func addIMAPModSeqOffset(modseq uint64, offset int64) uint64 {
	if offset <= 0 {
		return modseq
	}
	if ^uint64(0)-modseq < uint64(offset) {
		return ^uint64(0)
	}
	return modseq + uint64(offset)
}

type imapMessageRow struct {
	ID           string
	MailboxID    string
	RFCMessageID string
	Subject      string
	FromAddr     string
	FromName     string
	ToAddrs      json.RawMessage
	CcAddrs      json.RawMessage
	BccAddrs     json.RawMessage
	InternalDate time.Time
	Size         int64
	Read         bool
	Starred      bool
	Answered     bool
	Forwarded    bool
	Draft        bool
	Deleted      bool
	Keywords     imapKeywordList
	Status       string
}

type imapStoreFlagChanges struct {
	Read      *bool
	Starred   *bool
	Answered  *bool
	Forwarded *bool
	Deleted   *bool
	Keywords  imapKeywordList
	Mode      imapgw.StoreFlagsMode
}

func newIMAPStoreFlagChanges(flags imapgw.MessageFlags, mode imapgw.StoreFlagsMode) (imapStoreFlagChanges, error) {
	if flags.Draft || strings.TrimSpace(flags.Status) != "" {
		return imapStoreFlagChanges{}, fmt.Errorf("unsupported imap store flag set")
	}
	keywords, err := canonicalMailDBIMAPKeywords(flags.Keywords)
	if err != nil {
		return imapStoreFlagChanges{}, err
	}
	changes := imapStoreFlagChanges{Keywords: keywords, Mode: mode}
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
		if flags.Forwarded {
			changes.Forwarded = boolPointer(true)
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
		if flags.Forwarded {
			changes.Forwarded = boolPointer(false)
		}
		if flags.Deleted {
			changes.Deleted = boolPointer(false)
		}
	case imapgw.StoreFlagsReplace:
		changes.Read = boolPointer(flags.Read)
		changes.Starred = boolPointer(flags.Starred)
		changes.Answered = boolPointer(flags.Answered)
		changes.Forwarded = boolPointer(flags.Forwarded)
		changes.Deleted = boolPointer(flags.Deleted)
	default:
		return imapStoreFlagChanges{}, fmt.Errorf("unsupported imap store flags mode %q", mode)
	}
	if changes.Read == nil && changes.Starred == nil && changes.Answered == nil && changes.Forwarded == nil && changes.Deleted == nil && len(changes.Keywords) == 0 && mode != imapgw.StoreFlagsReplace {
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
	if changes.Forwarded != nil {
		next.Forwarded = *changes.Forwarded
	}
	if changes.Deleted != nil {
		next.Deleted = *changes.Deleted
	}
	switch changes.Mode {
	case imapgw.StoreFlagsAdd:
		next.Keywords = addIMAPKeywords(row.Keywords, changes.Keywords)
	case imapgw.StoreFlagsRemove:
		next.Keywords = removeIMAPKeywords(row.Keywords, changes.Keywords)
	case imapgw.StoreFlagsReplace:
		next.Keywords = append(imapKeywordList(nil), changes.Keywords...)
	}
	return next, next.Read != row.Read || next.Starred != row.Starred || next.Answered != row.Answered || next.Forwarded != row.Forwarded || next.Deleted != row.Deleted || !imapKeywordListsEqual(next.Keywords, row.Keywords)
}

type imapKeywordList []string

func (keywords *imapKeywordList) Scan(value any) error {
	if value == nil {
		*keywords = nil
		return nil
	}
	var raw []byte
	switch typed := value.(type) {
	case []byte:
		raw = typed
	case string:
		raw = []byte(typed)
	default:
		return fmt.Errorf("scan imap keywords: unsupported value type %T", value)
	}
	var parsed []string
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("scan imap keywords: %w", err)
	}
	canonical, err := canonicalMailDBIMAPKeywords(parsed)
	if err != nil {
		return err
	}
	*keywords = canonical
	return nil
}

func canonicalMailDBIMAPKeywords(keywords []string) (imapKeywordList, error) {
	if len(keywords) == 0 {
		return nil, nil
	}
	out := make(imapKeywordList, 0, len(keywords))
	seen := make(map[string]struct{}, len(keywords))
	for _, keyword := range keywords {
		canonical := imapgw.CanonicalIMAPFlag(keyword)
		if !imapgw.IMAPKeywordFlagValid(canonical) {
			return nil, fmt.Errorf("invalid imap keyword flag %q", keyword)
		}
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
	}
	return out, nil
}

func addIMAPKeywords(existing imapKeywordList, additions imapKeywordList) imapKeywordList {
	if len(additions) == 0 {
		return append(imapKeywordList(nil), existing...)
	}
	out := append(imapKeywordList(nil), existing...)
	seen := make(map[string]struct{}, len(existing)+len(additions))
	for _, keyword := range existing {
		seen[keyword] = struct{}{}
	}
	for _, keyword := range additions {
		if _, ok := seen[keyword]; ok {
			continue
		}
		seen[keyword] = struct{}{}
		out = append(out, keyword)
	}
	return out
}

func removeIMAPKeywords(existing imapKeywordList, removals imapKeywordList) imapKeywordList {
	if len(existing) == 0 || len(removals) == 0 {
		return append(imapKeywordList(nil), existing...)
	}
	remove := make(map[string]struct{}, len(removals))
	for _, keyword := range removals {
		remove[keyword] = struct{}{}
	}
	out := make(imapKeywordList, 0, len(existing))
	for _, keyword := range existing {
		if _, ok := remove[keyword]; ok {
			continue
		}
		out = append(out, keyword)
	}
	return out
}

func imapKeywordListsEqual(a imapKeywordList, b imapKeywordList) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
			To:        imapEnvelopeAddresses(row.ToAddrs),
			Cc:        imapEnvelopeAddresses(row.CcAddrs),
			Bcc:       imapEnvelopeAddresses(row.BccAddrs),
			Date:      row.InternalDate,
		},
		Flags: imapgw.MessageFlags{
			Read:      row.Read,
			Starred:   row.Starred,
			Answered:  row.Answered,
			Forwarded: row.Forwarded,
			Draft:     row.Draft,
			Deleted:   row.Deleted,
			Keywords:  append([]string(nil), row.Keywords...),
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

func imapSequenceBaseForAfterUID(ctx context.Context, querier imapSequenceQuerier, userID string, mailboxID string, afterUID imapgw.UID) (uint32, error) {
	if afterUID == 0 {
		return 0, nil
	}
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
	if err := querier.QueryRowContext(ctx, query, userID, mailboxID, int64(afterUID)).Scan(&count); err != nil {
		return 0, fmt.Errorf("get imap sequence base: %w", err)
	}
	if count < 0 || count > math.MaxUint32 {
		return 0, fmt.Errorf("imap sequence base unavailable after uid %d", afterUID)
	}
	return uint32(count), nil
}

func assignIMAPListSequenceNumbers(messages []imapgw.MessageSummary, base uint32) error {
	for i := range messages {
		if base > math.MaxUint32-uint32(i)-1 {
			return fmt.Errorf("imap sequence number overflow")
		}
		messages[i].SequenceNumber = base + uint32(i) + 1
	}
	return nil
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

func imapEnvelopeAddresses(raw json.RawMessage) []imapgw.Address {
	if len(raw) == 0 {
		return nil
	}
	var items []struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	addresses := make([]imapgw.Address, 0, len(items))
	for _, item := range items {
		addresses = append(addresses, imapEnvelopeAddress(item.Name, item.Address)...)
	}
	return addresses
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
	keywords, err := json.Marshal([]string(row.Keywords))
	if err != nil {
		return fmt.Errorf("marshal imap message keywords: %w", err)
	}
	const updateMessage = `
UPDATE messages
SET flags = jsonb_set(
      jsonb_set(
        jsonb_set(
          jsonb_set(
            jsonb_set(
              jsonb_set(COALESCE(flags, '{}'::jsonb), '{read}', to_jsonb($2::boolean), true),
              '{starred}', to_jsonb($3::boolean), true
            ),
            '{answered}', to_jsonb($4::boolean), true
          ),
          '{forwarded}', to_jsonb($5::boolean), true
        ),
        '{imap_deleted}', to_jsonb($6::boolean), true
      ),
      '{imap_keywords}', $7::jsonb, true
    ),
    updated_at = now()
WHERE id = $1::uuid`
	if _, err := tx.ExecContext(ctx, updateMessage, messageID, row.Read, row.Starred, row.Answered, row.Forwarded, row.Deleted, string(keywords)); err != nil {
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
	if userID == "" {
		return IMAPUIDState{}, fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(mailboxID) == "" {
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
