package maildb

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ThreadSummary struct {
	ID              string    `json:"id"`
	Subject         string    `json:"subject"`
	MessageCount    int64     `json:"message_count"`
	UnreadCount     int64     `json:"unread_count"`
	LatestMessageID string    `json:"latest_message_id"`
	LatestFromAddr  string    `json:"latest_from_addr"`
	LatestAt        time.Time `json:"latest_at"`
	HasAttachment   bool      `json:"has_attachment"`
	Starred         bool      `json:"starred"`
}

func (r *Repository) ListThreads(ctx context.Context, userID string, limit int) ([]ThreadSummary, error) {
	return r.ListThreadsPage(ctx, userID, limit, ThreadListCursor{}, ThreadListFilter{})
}

func (r *Repository) ListThreadsPage(ctx context.Context, userID string, limit int, cursor ThreadListCursor, filter ThreadListFilter) ([]ThreadSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	limit = normalizeLimit(limit) + 1
	sortMode, ok := NormalizeListSort(filter.Sort)
	if !ok {
		return nil, fmt.Errorf("unsupported list sort %q", filter.Sort)
	}

	query := threadListPageNewestSQL
	if sortMode == ListSortOldest {
		query = threadListPageOldestSQL
	}

	rows, err := r.db.QueryContext(ctx, query, userID, limit, cursor.At, strings.TrimSpace(cursor.ID), filter.Read, filter.Starred, filter.HasAttachment, strings.TrimSpace(filter.FolderID))
	if err != nil {
		return nil, fmt.Errorf("list threads: %w", err)
	}
	defer rows.Close()

	var threads []ThreadSummary
	for rows.Next() {
		var thread ThreadSummary
		if err := rows.Scan(
			&thread.ID,
			&thread.Subject,
			&thread.MessageCount,
			&thread.UnreadCount,
			&thread.LatestMessageID,
			&thread.LatestFromAddr,
			&thread.LatestAt,
			&thread.HasAttachment,
			&thread.Starred,
		); err != nil {
			return nil, fmt.Errorf("scan thread summary: %w", err)
		}
		threads = append(threads, thread)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thread summaries: %w", err)
	}
	return threads, nil
}

const threadListPageNewestSQL = `
WITH active_messages AS (
  SELECT
    COALESCE(thread_id, id)::text AS thread_key,
    id::text AS id,
    subject,
    from_addr,
    COALESCE(received_at, sent_at, draft_updated_at, created_at) AS message_at,
    has_attachment,
    COALESCE((flags->>'read')::boolean, false) AS read,
    COALESCE((flags->>'starred')::boolean, false) AS starred
  FROM messages
  WHERE user_id = $1
    AND status = 'active'
    AND ($8 = '' OR folder_id::text = $8)
),
thread_summaries AS (
SELECT
  thread_key,
  (array_agg(subject ORDER BY message_at DESC, id DESC))[1] AS subject,
  count(*) AS message_count,
  count(*) FILTER (WHERE read = false) AS unread_count,
  (array_agg(id ORDER BY message_at DESC, id DESC))[1] AS latest_message_id,
  (array_agg(from_addr ORDER BY message_at DESC, id DESC))[1] AS latest_from_addr,
  max(message_at) AS latest_at,
  bool_or(has_attachment) AS has_attachment,
  bool_or(starred) AS starred
FROM active_messages
GROUP BY thread_key
)
SELECT *
FROM thread_summaries
WHERE (
  $4 = ''
  OR (latest_at, thread_key) < ($3::timestamptz, $4)
)
AND (
  $5::boolean IS NULL
  OR ($5::boolean = false AND unread_count > 0)
  OR ($5::boolean = true AND unread_count = 0)
)
AND ($6::boolean IS NULL OR starred = $6::boolean)
AND ($7::boolean IS NULL OR has_attachment = $7::boolean)
ORDER BY latest_at DESC, thread_key DESC
LIMIT $2`

const threadListPageOldestSQL = `
WITH active_messages AS (
  SELECT
    COALESCE(thread_id, id)::text AS thread_key,
    id::text AS id,
    subject,
    from_addr,
    COALESCE(received_at, sent_at, draft_updated_at, created_at) AS message_at,
    has_attachment,
    COALESCE((flags->>'read')::boolean, false) AS read,
    COALESCE((flags->>'starred')::boolean, false) AS starred
  FROM messages
  WHERE user_id = $1
    AND status = 'active'
    AND ($8 = '' OR folder_id::text = $8)
),
thread_summaries AS (
SELECT
  thread_key,
  (array_agg(subject ORDER BY message_at DESC, id DESC))[1] AS subject,
  count(*) AS message_count,
  count(*) FILTER (WHERE read = false) AS unread_count,
  (array_agg(id ORDER BY message_at DESC, id DESC))[1] AS latest_message_id,
  (array_agg(from_addr ORDER BY message_at DESC, id DESC))[1] AS latest_from_addr,
  max(message_at) AS latest_at,
  bool_or(has_attachment) AS has_attachment,
  bool_or(starred) AS starred
FROM active_messages
GROUP BY thread_key
)
SELECT *
FROM thread_summaries
WHERE (
  $4 = ''
  OR (latest_at, thread_key) > ($3::timestamptz, $4)
)
AND (
  $5::boolean IS NULL
  OR ($5::boolean = false AND unread_count > 0)
  OR ($5::boolean = true AND unread_count = 0)
)
AND ($6::boolean IS NULL OR starred = $6::boolean)
AND ($7::boolean IS NULL OR has_attachment = $7::boolean)
ORDER BY latest_at ASC, thread_key ASC
LIMIT $2`

func (r *Repository) ListThreadMessages(ctx context.Context, userID string, threadID string, limit int) ([]MessageSummary, error) {
	return r.ListThreadMessagesPage(ctx, userID, threadID, limit, MessageListCursor{})
}

func (r *Repository) ListThreadMessagesPage(ctx context.Context, userID string, threadID string, limit int, cursor MessageListCursor) ([]MessageSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	threadID = strings.TrimSpace(threadID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if threadID == "" {
		return nil, fmt.Errorf("thread id is required")
	}
	limit = normalizeLimit(limit) + 1

	const query = `
SELECT
  id::text,
  subject,
  from_addr,
  from_name,
  COALESCE(received_at, sent_at, draft_updated_at, created_at) AS message_at,
  size,
  has_attachment,
  COALESCE((flags->>'read')::boolean, false) AS read,
  COALESCE((flags->>'starred')::boolean, false) AS starred
FROM messages
WHERE user_id = $1
  AND status = 'active'
  AND COALESCE(thread_id, id)::text = $2
  AND (
    $5 = ''
    OR (COALESCE(received_at, sent_at, draft_updated_at, created_at), id)
       > ($4::timestamptz, $5::uuid)
  )
ORDER BY message_at ASC, id ASC
LIMIT $3`

	rows, err := r.db.QueryContext(ctx, query, userID, threadID, limit, cursor.At, strings.TrimSpace(cursor.ID))
	if err != nil {
		return nil, fmt.Errorf("list thread messages: %w", err)
	}
	defer rows.Close()

	var messages []MessageSummary
	for rows.Next() {
		var msg MessageSummary
		if err := rows.Scan(
			&msg.ID,
			&msg.Subject,
			&msg.FromAddr,
			&msg.FromName,
			&msg.ReceivedAt,
			&msg.Size,
			&msg.HasAttachment,
			&msg.Read,
			&msg.Starred,
		); err != nil {
			return nil, fmt.Errorf("scan thread message: %w", err)
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thread messages: %w", err)
	}
	return messages, nil
}
