package maildb

import (
	"context"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

func (r *Repository) BulkSetMessageFlag(ctx context.Context, req BulkMessageFlagRequest) (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkMessageFlagRequest(req); err != nil {
		return 0, err
	}
	flag := strings.TrimSpace(req.Flag)

	const query = `
UPDATE messages
SET flags = jsonb_set(COALESCE(flags, '{}'::jsonb), $3::text[], to_jsonb($4::boolean), true),
    updated_at = now()
WHERE user_id = $1
  AND id IN (SELECT value FROM unnest($2::uuid[]) AS requested(value))
  AND status = 'active'`

	result, err := r.db.ExecContext(ctx, query, strings.TrimSpace(req.UserID), pq.Array(req.MessageIDs), "{"+flag+"}", req.Value)
	if err != nil {
		return 0, fmt.Errorf("bulk set message flag: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("inspect bulk message flag update: %w", err)
	}
	return affected, nil
}

func (r *Repository) BulkSetThreadFlag(ctx context.Context, req BulkThreadFlagRequest) (BulkThreadFlagResult, error) {
	if r.db == nil {
		return BulkThreadFlagResult{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkThreadFlagRequest(req); err != nil {
		return BulkThreadFlagResult{}, err
	}
	flag := strings.TrimSpace(req.Flag)

	rows, err := r.db.QueryContext(ctx, bulkSetThreadFlagSQL, strings.TrimSpace(req.UserID), pq.Array(req.ThreadIDs), "{"+flag+"}", req.Value)
	if err != nil {
		return BulkThreadFlagResult{}, fmt.Errorf("bulk set thread flag: %w", err)
	}
	defer rows.Close()

	var messageIDs []string
	for rows.Next() {
		var messageID string
		if err := rows.Scan(&messageID); err != nil {
			return BulkThreadFlagResult{}, fmt.Errorf("scan bulk thread flag message: %w", err)
		}
		messageIDs = append(messageIDs, messageID)
	}
	if err := rows.Err(); err != nil {
		return BulkThreadFlagResult{}, fmt.Errorf("iterate bulk thread flag messages: %w", err)
	}
	return BulkThreadFlagResult{Updated: int64(len(messageIDs)), MessageIDs: messageIDs}, nil
}

const bulkSetThreadFlagSQL = `
WITH requested AS (
  SELECT value AS id
  FROM unnest($2::uuid[]) AS requested(value)
),
target_messages AS (
  SELECT messages.id
  FROM messages
  JOIN requested ON messages.thread_id = requested.id
  WHERE user_id = $1
    AND status = 'active'
  UNION
  SELECT messages.id
  FROM messages
  JOIN requested ON messages.id = requested.id
  WHERE user_id = $1
    AND status = 'active'
),
updated_messages AS (
  UPDATE messages
  SET flags = jsonb_set(COALESCE(flags, '{}'::jsonb), $3::text[], to_jsonb($4::boolean), true),
      updated_at = now()
  WHERE id IN (SELECT id FROM target_messages)
  RETURNING id::text
)
SELECT id
FROM updated_messages
ORDER BY id`

func (r *Repository) ListMessageIDsForThreads(ctx context.Context, userID string, threadIDs []string) ([]string, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if err := validateBulkThreadIDs(threadIDs); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, listMessageIDsForThreadsSQL, userID, pq.Array(threadIDs))
	if err != nil {
		return nil, fmt.Errorf("list message ids for threads: %w", err)
	}
	defer rows.Close()

	var messageIDs []string
	for rows.Next() {
		var messageID string
		if err := rows.Scan(&messageID); err != nil {
			return nil, fmt.Errorf("scan thread message id: %w", err)
		}
		messageIDs = append(messageIDs, messageID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thread message ids: %w", err)
	}
	return messageIDs, nil
}

const listMessageIDsForThreadsSQL = `
WITH requested AS (
  SELECT value AS id
  FROM unnest($2::uuid[]) AS requested(value)
)
SELECT id::text
FROM messages
JOIN requested ON messages.thread_id = requested.id
WHERE user_id = $1
  AND status = 'active'
UNION
SELECT id::text
FROM messages
JOIN requested ON messages.id = requested.id
WHERE user_id = $1
  AND status = 'active'
ORDER BY id`

func (r *Repository) BulkMoveMessages(ctx context.Context, req BulkMessageMoveRequest) (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkMessageMoveRequest(req); err != nil {
		return 0, err
	}
	userID := strings.TrimSpace(req.UserID)
	folderID := strings.TrimSpace(req.FolderID)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin bulk move transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
UPDATE messages
SET folder_id = $3,
    updated_at = now()
WHERE user_id = $1
  AND id IN (SELECT value FROM unnest($2::uuid[]) AS requested(value))
  AND status = 'active'
  AND EXISTS (
    SELECT 1
    FROM folders
    WHERE folders.id = $3
      AND folders.user_id = $1
  )
RETURNING id::text`

	rows, err := tx.QueryContext(ctx, query, userID, pq.Array(req.MessageIDs), folderID)
	if err != nil {
		return 0, fmt.Errorf("bulk move messages: %w", err)
	}
	var movedIDs []string
	for rows.Next() {
		var movedID string
		if err := rows.Scan(&movedID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan bulk moved message: %w", err)
		}
		movedIDs = append(movedIDs, movedID)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close bulk moved message rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate bulk moved messages: %w", err)
	}
	if err := deleteIMAPUIDRowsForMessages(ctx, tx, userID, movedIDs); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit bulk move transaction: %w", err)
	}
	return int64(len(movedIDs)), nil
}

func (r *Repository) BulkMoveThreads(ctx context.Context, req BulkThreadMoveRequest) (BulkThreadMoveResult, error) {
	if r.db == nil {
		return BulkThreadMoveResult{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkThreadMoveRequest(req); err != nil {
		return BulkThreadMoveResult{}, err
	}
	userID := strings.TrimSpace(req.UserID)
	folderID := strings.TrimSpace(req.FolderID)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BulkThreadMoveResult{}, fmt.Errorf("begin bulk thread move transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, bulkMoveThreadsSQL, userID, pq.Array(req.ThreadIDs), folderID)
	if err != nil {
		return BulkThreadMoveResult{}, fmt.Errorf("bulk move threads: %w", err)
	}
	var movedIDs []string
	for rows.Next() {
		var movedID string
		if err := rows.Scan(&movedID); err != nil {
			rows.Close()
			return BulkThreadMoveResult{}, fmt.Errorf("scan bulk moved thread message: %w", err)
		}
		movedIDs = append(movedIDs, movedID)
	}
	if err := rows.Close(); err != nil {
		return BulkThreadMoveResult{}, fmt.Errorf("close bulk moved thread rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return BulkThreadMoveResult{}, fmt.Errorf("iterate bulk moved thread messages: %w", err)
	}
	if err := deleteIMAPUIDRowsForMessages(ctx, tx, userID, movedIDs); err != nil {
		return BulkThreadMoveResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return BulkThreadMoveResult{}, fmt.Errorf("commit bulk thread move transaction: %w", err)
	}
	return BulkThreadMoveResult{Updated: int64(len(movedIDs)), MessageIDs: movedIDs}, nil
}

const bulkMoveThreadsSQL = `
WITH requested AS (
  SELECT value AS id
  FROM unnest($2::uuid[]) AS requested(value)
),
target_messages AS (
  SELECT messages.id
  FROM messages
  JOIN requested ON messages.thread_id = requested.id
  WHERE user_id = $1
    AND status = 'active'
  UNION
  SELECT messages.id
  FROM messages
  JOIN requested ON messages.id = requested.id
  WHERE user_id = $1
    AND status = 'active'
),
updated_messages AS (
  UPDATE messages
  SET folder_id = $3,
      updated_at = now()
  WHERE id IN (SELECT id FROM target_messages)
    AND EXISTS (
    SELECT 1
    FROM folders
    WHERE folders.id = $3
      AND folders.user_id = $1
  )
  RETURNING id::text
)
SELECT id
FROM updated_messages
ORDER BY id`

func (r *Repository) BulkDeleteMessages(ctx context.Context, req BulkMessageDeleteRequest) (int64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkMessageDeleteRequest(req); err != nil {
		return 0, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin bulk delete transaction: %w", err)
	}
	defer tx.Rollback()

	var totalSize int64
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(size), 0)
FROM messages
WHERE user_id = $1
  AND id IN (SELECT value FROM unnest($2::uuid[]) AS requested(value))
  AND status = 'active'`, strings.TrimSpace(req.UserID), pq.Array(req.MessageIDs),
	).Scan(&totalSize); err != nil {
		return 0, fmt.Errorf("sum message sizes for bulk delete: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
UPDATE messages
SET status = 'deleted',
    deleted_at = now(),
    updated_at = now()
WHERE user_id = $1
  AND id IN (SELECT value FROM unnest($2::uuid[]) AS requested(value))
  AND status = 'active'`, strings.TrimSpace(req.UserID), pq.Array(req.MessageIDs))
	if err != nil {
		return 0, fmt.Errorf("bulk delete messages: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("inspect bulk message delete: %w", err)
	}

	if err := deleteIMAPUIDRowsForMessages(ctx, tx, strings.TrimSpace(req.UserID), req.MessageIDs); err != nil {
		return 0, err
	}
	if err := decrementUserQuota(ctx, tx, strings.TrimSpace(req.UserID), totalSize); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit bulk delete transaction: %w", err)
	}
	return affected, nil
}

func (r *Repository) BulkRestoreMessages(ctx context.Context, req BulkMessageRestoreRequest) (BulkMessageRestoreResult, error) {
	if r.db == nil {
		return BulkMessageRestoreResult{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkMessageRestoreRequest(req); err != nil {
		return BulkMessageRestoreResult{}, err
	}
	userID := strings.TrimSpace(req.UserID)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BulkMessageRestoreResult{}, fmt.Errorf("begin bulk restore transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
SELECT id::text, COALESCE(size, 0)
FROM messages
WHERE user_id = $1
  AND id IN (SELECT value FROM unnest($2::uuid[]) AS requested(value))
  AND status = 'deleted'
FOR UPDATE`, userID, pq.Array(req.MessageIDs))
	if err != nil {
		return BulkMessageRestoreResult{}, fmt.Errorf("list messages for bulk restore: %w", err)
	}
	var restoredIDs []string
	var totalSize int64
	for rows.Next() {
		var id string
		var size int64
		if err := rows.Scan(&id, &size); err != nil {
			rows.Close()
			return BulkMessageRestoreResult{}, fmt.Errorf("scan message for bulk restore: %w", err)
		}
		restoredIDs = append(restoredIDs, id)
		totalSize += size
	}
	if err := rows.Close(); err != nil {
		return BulkMessageRestoreResult{}, fmt.Errorf("close bulk restore rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return BulkMessageRestoreResult{}, fmt.Errorf("iterate bulk restore rows: %w", err)
	}
	if err := checkAndIncrementUserQuota(ctx, tx, userID, totalSize); err != nil {
		return BulkMessageRestoreResult{}, err
	}

	if len(restoredIDs) > 0 {
		if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET status = 'active',
    deleted_at = NULL,
    updated_at = now()
WHERE user_id = $1
  AND id IN (SELECT value FROM unnest($2::uuid[]) AS requested(value))
  AND status = 'deleted'`, userID, pq.Array(restoredIDs)); err != nil {
			return BulkMessageRestoreResult{}, fmt.Errorf("bulk restore messages: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return BulkMessageRestoreResult{}, fmt.Errorf("commit bulk restore transaction: %w", err)
	}
	return BulkMessageRestoreResult{Updated: int64(len(restoredIDs)), MessageIDs: restoredIDs}, nil
}

func (r *Repository) BulkRestoreThreads(ctx context.Context, req BulkThreadRestoreRequest) (BulkThreadRestoreResult, error) {
	if r.db == nil {
		return BulkThreadRestoreResult{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkThreadRestoreRequest(req); err != nil {
		return BulkThreadRestoreResult{}, err
	}
	userID := strings.TrimSpace(req.UserID)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BulkThreadRestoreResult{}, fmt.Errorf("begin bulk thread restore transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, bulkRestoreThreadsSQL, userID, pq.Array(req.ThreadIDs))
	if err != nil {
		return BulkThreadRestoreResult{}, fmt.Errorf("bulk restore threads: %w", err)
	}
	var restoredIDs []string
	var totalSize int64
	for rows.Next() {
		var restoredID string
		var size int64
		if err := rows.Scan(&restoredID, &size); err != nil {
			rows.Close()
			return BulkThreadRestoreResult{}, fmt.Errorf("scan bulk restored thread message: %w", err)
		}
		restoredIDs = append(restoredIDs, restoredID)
		totalSize += size
	}
	if err := rows.Close(); err != nil {
		return BulkThreadRestoreResult{}, fmt.Errorf("close bulk restored thread rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return BulkThreadRestoreResult{}, fmt.Errorf("iterate bulk restored thread messages: %w", err)
	}
	if err := checkAndIncrementUserQuota(ctx, tx, userID, totalSize); err != nil {
		return BulkThreadRestoreResult{}, err
	}
	if len(restoredIDs) > 0 {
		if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET status = 'active',
    deleted_at = NULL,
    updated_at = now()
WHERE user_id = $1
  AND id IN (SELECT value FROM unnest($2::uuid[]) AS requested(value))
  AND status = 'deleted'`, userID, pq.Array(restoredIDs)); err != nil {
			return BulkThreadRestoreResult{}, fmt.Errorf("activate bulk restored thread messages: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return BulkThreadRestoreResult{}, fmt.Errorf("commit bulk thread restore transaction: %w", err)
	}
	return BulkThreadRestoreResult{Updated: int64(len(restoredIDs)), MessageIDs: restoredIDs}, nil
}

const bulkRestoreThreadsSQL = `
WITH requested AS (
  SELECT value AS id
  FROM unnest($2::uuid[]) AS requested(value)
),
target_messages AS (
  SELECT messages.id
  FROM messages
  JOIN requested ON messages.thread_id = requested.id
  WHERE user_id = $1
    AND status = 'deleted'
  UNION
  SELECT messages.id
  FROM messages
  JOIN requested ON messages.id = requested.id
  WHERE user_id = $1
    AND status = 'deleted'
)
SELECT messages.id::text, COALESCE(messages.size, 0)
FROM messages
JOIN target_messages ON target_messages.id = messages.id
FOR UPDATE`

func (r *Repository) BulkDeleteThreads(ctx context.Context, req BulkThreadDeleteRequest) (BulkThreadDeleteResult, error) {
	if r.db == nil {
		return BulkThreadDeleteResult{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkThreadDeleteRequest(req); err != nil {
		return BulkThreadDeleteResult{}, err
	}
	userID := strings.TrimSpace(req.UserID)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BulkThreadDeleteResult{}, fmt.Errorf("begin bulk thread delete transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, bulkDeleteThreadsSQL, userID, pq.Array(req.ThreadIDs))
	if err != nil {
		return BulkThreadDeleteResult{}, fmt.Errorf("bulk delete threads: %w", err)
	}
	var deletedIDs []string
	var totalSize int64
	for rows.Next() {
		var deletedID string
		var size int64
		if err := rows.Scan(&deletedID, &size); err != nil {
			rows.Close()
			return BulkThreadDeleteResult{}, fmt.Errorf("scan bulk deleted thread message: %w", err)
		}
		deletedIDs = append(deletedIDs, deletedID)
		totalSize += size
	}
	if err := rows.Close(); err != nil {
		return BulkThreadDeleteResult{}, fmt.Errorf("close bulk deleted thread rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return BulkThreadDeleteResult{}, fmt.Errorf("iterate bulk deleted thread messages: %w", err)
	}
	if err := deleteIMAPUIDRowsForMessages(ctx, tx, userID, deletedIDs); err != nil {
		return BulkThreadDeleteResult{}, err
	}
	if err := decrementUserQuota(ctx, tx, userID, totalSize); err != nil {
		return BulkThreadDeleteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return BulkThreadDeleteResult{}, fmt.Errorf("commit bulk thread delete transaction: %w", err)
	}
	return BulkThreadDeleteResult{Updated: int64(len(deletedIDs)), MessageIDs: deletedIDs}, nil
}

const bulkDeleteThreadsSQL = `
WITH requested AS (
  SELECT value AS id
  FROM unnest($2::uuid[]) AS requested(value)
),
target_messages AS (
  SELECT messages.id
  FROM messages
  JOIN requested ON messages.thread_id = requested.id
  WHERE user_id = $1
    AND status = 'active'
  UNION
  SELECT messages.id
  FROM messages
  JOIN requested ON messages.id = requested.id
  WHERE user_id = $1
    AND status = 'active'
),
deleted_messages AS (
  UPDATE messages
  SET status = 'deleted',
      deleted_at = now(),
      updated_at = now()
  WHERE id IN (SELECT id FROM target_messages)
  RETURNING id::text, COALESCE(size, 0) AS size
)
SELECT id, size
FROM deleted_messages
ORDER BY id`
