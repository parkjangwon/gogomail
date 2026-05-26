package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/lib/pq"
)

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
