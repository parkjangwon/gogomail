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
	Preview         string    `json:"preview"`
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

	folderID := strings.TrimSpace(filter.FolderID)
	cursorID := strings.TrimSpace(cursor.ID)
	query := buildThreadListPageSQL(sortMode, folderID, cursorID, filter)

	rows, err := r.db.QueryContext(
		ctx,
		query,
		userID,
		limit,
		optionalTimeParam(cursorID != "", cursor.At),
		optionalStringParam(cursorID),
		optionalBoolParam(filter.Read),
		optionalBoolParam(filter.Starred),
		optionalBoolParam(filter.HasAttachment),
		optionalStringParam(folderID),
	)
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
			&thread.Preview,
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

func buildThreadListPageSQL(sortMode, folderID, cursorID string, filter ThreadListFilter) string {
	activeConditions := []string{
		"messages.user_id = $1",
		"messages.status = 'active'",
	}
	if folderID != "" {
		activeConditions = append(activeConditions, "messages.folder_id = $8::uuid")
	}
	summaryConditions := []string{"TRUE"}
	cursorOp := "<"
	orderBy := "ORDER BY latest_at DESC, thread_key DESC"
	if sortMode == ListSortOldest {
		cursorOp = ">"
		orderBy = "ORDER BY latest_at ASC, thread_key ASC"
	}
	if cursorID != "" {
		summaryConditions[0] = fmt.Sprintf("(latest_at, thread_key) %s ($3::timestamptz, $4)", cursorOp)
	}
	if filter.Read != nil {
		if *filter.Read {
			summaryConditions = append(summaryConditions, "unread_count = 0")
		} else {
			summaryConditions = append(summaryConditions, "unread_count > 0")
		}
	}
	if filter.Starred != nil {
		summaryConditions = append(summaryConditions, "starred = $6::boolean")
	}
	if filter.HasAttachment != nil {
		summaryConditions = append(summaryConditions, "has_attachment = $7::boolean")
	}

	return fmt.Sprintf(threadListPageBaseSQL,
		strings.Join(activeConditions, "\n    AND "),
		strings.Join(summaryConditions, "\nAND "),
		orderBy,
	)
}

var (
	threadListPageNewestSQL = buildThreadListPageSQL(ListSortNewest, "", "", ThreadListFilter{})
	threadListPageOldestSQL = buildThreadListPageSQL(ListSortOldest, "", "", ThreadListFilter{})
)

const threadListPageBaseSQL = `
WITH thread_list_page_params AS (
  SELECT
    $3::timestamptz AS cursor_at,
    $4::text AS cursor_id,
    $5::boolean AS read_filter,
    $6::boolean AS starred_filter,
    $7::boolean AS has_attachment_filter,
    $8::uuid AS folder_id
),
active_messages AS (
  SELECT
    COALESCE(messages.thread_id, messages.id)::text AS thread_key,
    messages.id::text AS id,
    messages.subject,
    left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview,
    messages.from_addr,
    COALESCE(messages.received_at, messages.sent_at, messages.draft_updated_at, messages.created_at) AS message_at,
    messages.has_attachment,
    COALESCE((messages.flags->>'read')::boolean, false) AS read,
    COALESCE((messages.flags->>'starred')::boolean, false) AS starred
  FROM messages
  CROSS JOIN thread_list_page_params
  LEFT JOIN message_search_documents msd
    ON msd.message_id = messages.id
   AND msd.user_id = messages.user_id
  WHERE %s
),
thread_summaries AS (
SELECT
  thread_key,
  (array_agg(subject ORDER BY message_at DESC, id DESC))[1] AS subject,
  (array_agg(preview ORDER BY message_at DESC, id DESC))[1] AS preview,
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
SELECT
  thread_key,
  subject,
  preview,
  message_count,
  unread_count,
  latest_message_id,
  latest_from_addr,
  latest_at,
  has_attachment,
  starred
FROM thread_summaries
WHERE %s
%s
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

	const query = threadMessagesPageSQL

	rows, err := r.db.QueryContext(
		ctx,
		query,
		userID,
		threadID,
		limit,
		cursor.At,
		strings.TrimSpace(cursor.ID),
	)
	if err != nil {
		return nil, fmt.Errorf("list thread messages: %w", err)
	}
	defer rows.Close()

	var messages []MessageSummary
	for rows.Next() {
		var msg MessageSummary
		if err := rows.Scan(
			&msg.ID,
			&msg.FolderID,
			&msg.Subject,
			&msg.Preview,
			&msg.FromAddr,
			&msg.FromName,
			&msg.SenderAvatarURL,
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

const threadMessagesPageSQL = `
SELECT
  messages.id::text,
  messages.folder_id::text,
  messages.subject,
  left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview,
  messages.from_addr,
  messages.from_name,
  COALESCE(sender_user.settings->>'avatar_url', '') AS sender_avatar_url,
  COALESCE(messages.received_at, messages.sent_at, messages.draft_updated_at, messages.created_at) AS message_at,
  messages.size,
  messages.has_attachment,
  COALESCE((messages.flags->>'read')::boolean, false) AS read,
  COALESCE((messages.flags->>'starred')::boolean, false) AS starred
FROM messages
LEFT JOIN message_search_documents msd
  ON msd.message_id = messages.id
 AND msd.user_id = messages.user_id
LEFT JOIN user_addresses sender_addr
  ON sender_addr.address_ace = lower(messages.from_addr)
LEFT JOIN users sender_user
  ON sender_user.id = sender_addr.user_id
 AND sender_user.status = 'active'
WHERE messages.user_id = $1
  AND messages.status = 'active'
  AND (
    messages.thread_id = $2::uuid
    OR messages.id = $2::uuid
  )
  AND (
    $5 = ''
    OR (COALESCE(messages.received_at, messages.sent_at, messages.draft_updated_at, messages.created_at), messages.id)
       > ($4::timestamptz, $5::uuid)
  )
ORDER BY message_at ASC, id ASC
LIMIT $3`
