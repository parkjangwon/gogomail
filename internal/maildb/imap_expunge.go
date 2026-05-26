package maildb

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/lib/pq"
)

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
