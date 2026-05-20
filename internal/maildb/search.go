package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const (
	MessageSearchSortDate      = "date"
	MessageSearchSortRelevance = "relevance"
)

type MessageSearchQuery struct {
	UserID            string
	Query             string
	FolderID          string
	From              string
	To                string
	Cc                string
	Bcc               string
	Subject           string
	HasAttachment     *bool
	Since             string
	Until             string
	Limit             int
	Sort              string
	Cursor            MessageListCursor
	IncludeRank       bool
	IncludeHighlights bool
}

type DraftSearchQuery struct {
	UserID        string
	Query         string
	From          string
	To            string
	Cc            string
	Bcc           string
	Subject       string
	HasAttachment *bool
	Limit         int
	Cursor        MessageListCursor
}

func (q MessageSearchQuery) normalizedLimit() int {
	return normalizeLimit(q.Limit)
}

func (q DraftSearchQuery) normalizedLimit() int {
	return normalizeLimit(q.Limit)
}

func (q MessageSearchQuery) normalizedSort() string {
	switch strings.ToLower(strings.TrimSpace(q.Sort)) {
	case "", MessageSearchSortDate:
		return MessageSearchSortDate
	case MessageSearchSortRelevance:
		return MessageSearchSortRelevance
	default:
		return ""
	}
}

func (r *Repository) SearchMessages(ctx context.Context, query MessageSearchQuery) ([]MessageSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID := strings.TrimSpace(query.UserID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	limit := query.normalizedLimit() + 1
	sortMode := query.normalizedSort()
	if sortMode == "" {
		return nil, fmt.Errorf("unsupported search sort %q", query.Sort)
	}
	hasAttachment := ""
	if query.HasAttachment != nil {
		if *query.HasAttachment {
			hasAttachment = "true"
		} else {
			hasAttachment = "false"
		}
	}

	folderID := strings.TrimSpace(query.FolderID)
	cursorID := strings.TrimSpace(query.Cursor.ID)
	sqlQuery := buildMessageSearchSQL(sortMode, folderID, hasAttachment, cursorID)
	rows, err := r.db.QueryContext(
		ctx,
		sqlQuery,
		userID,
		strings.TrimSpace(query.Query),
		folderID,
		strings.TrimSpace(query.From),
		strings.TrimSpace(query.To),
		strings.TrimSpace(query.Cc),
		strings.TrimSpace(query.Bcc),
		strings.TrimSpace(query.Subject),
		hasAttachment,
		limit,
		query.IncludeRank || sortMode == MessageSearchSortRelevance,
		query.IncludeHighlights,
		query.Cursor.At,
		cursorID,
	)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()

	messages := make([]MessageSummary, 0, limit)
	for rows.Next() {
		var msg MessageSummary
		var rank sql.NullFloat64
		var subjectHighlight, fromHighlight, bodyHighlight sql.NullString
		if err := rows.Scan(
			&msg.ID,
			&msg.FolderID,
			&msg.Subject,
			&msg.Preview,
			&msg.FromAddr,
			&msg.FromName,
			&msg.ReceivedAt,
			&msg.Size,
			&msg.HasAttachment,
			&msg.Read,
			&msg.Starred,
			&rank,
			&subjectHighlight,
			&fromHighlight,
			&bodyHighlight,
		); err != nil {
			return nil, fmt.Errorf("scan search message: %w", err)
		}
		if query.IncludeRank && rank.Valid {
			msg.SearchRank = &rank.Float64
		}
		msg.SearchHighlights = searchHighlightsFromSQL(subjectHighlight, fromHighlight, bodyHighlight)
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search messages: %w", err)
	}
	return messages, nil
}

func buildMessageSearchSQL(sortMode, folderID, hasAttachment, cursorID string) string {
	conditions := messageSearchBaseConditions()
	if folderID != "" {
		conditions = append(conditions, "folder_id = $3::uuid")
	}
	if hasAttachment != "" {
		conditions = append(conditions, "has_attachment = $9::boolean")
	}
	if cursorID != "" {
		conditions = append(conditions, "(COALESCE(received_at, sent_at, draft_updated_at, created_at), id) < ($13::timestamptz, $14::uuid)")
	}
	return messageSearchSQLWithConditions(sortMode, conditions)
}

func (r *Repository) SearchDrafts(ctx context.Context, query DraftSearchQuery) ([]MessageDetail, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID := strings.TrimSpace(query.UserID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	limit := query.normalizedLimit()
	hasAttachment := ""
	if query.HasAttachment != nil {
		if *query.HasAttachment {
			hasAttachment = "true"
		} else {
			hasAttachment = "false"
		}
	}

	cursorID := strings.TrimSpace(query.Cursor.ID)
	rows, err := r.db.QueryContext(
		ctx,
		buildDraftSearchSQL(hasAttachment, cursorID),
		userID,
		strings.TrimSpace(query.Query),
		strings.TrimSpace(query.From),
		strings.TrimSpace(query.To),
		strings.TrimSpace(query.Cc),
		strings.TrimSpace(query.Bcc),
		strings.TrimSpace(query.Subject),
		hasAttachment,
		query.Cursor.At,
		cursorID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search drafts: %w", err)
	}
	defer rows.Close()

	drafts := make([]MessageDetail, 0, limit)
	for rows.Next() {
		var draft MessageDetail
		if err := rows.Scan(
			&draft.ID,
			&draft.MessageID,
			&draft.Subject,
			&draft.FromAddr,
			&draft.FromName,
			&draft.ToAddrs,
			&draft.CcAddrs,
			&draft.BccAddrs,
			&draft.ReceivedAt,
			&draft.Size,
			&draft.HasAttachment,
			&draft.Flags,
			&draft.StoragePath,
			&draft.TextBody,
		); err != nil {
			return nil, fmt.Errorf("scan search draft: %w", err)
		}
		drafts = append(drafts, draft)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search drafts: %w", err)
	}
	return drafts, nil
}

func messageSearchSQL(sortMode string) string {
	return messageSearchSQLWithConditions(sortMode, messageSearchBaseConditions())
}

func messageSearchBaseConditions() []string {
	return []string{
		"messages.user_id = $1",
		"messages.status = 'active'",
		`($2 = '' OR (
    (
      setweight(to_tsvector('simple', coalesce(subject, '')), 'A') ||
      setweight(to_tsvector('simple', coalesce(from_addr, '')), 'A') ||
      setweight(to_tsvector('simple', coalesce(from_name, '')), 'B') ||
      setweight(to_tsvector('simple', coalesce(msd.body_text, '')), 'D')
    ) @@ plainto_tsquery('simple', $2)
    OR subject ILIKE '%' || $2 || '%'
    OR from_addr ILIKE '%' || $2 || '%'
    OR msd.body_text ILIKE '%' || $2 || '%'
  )`,
		"($4 = '' OR from_addr ILIKE '%' || $4 || '%')",
		`($5 = '' OR (
    to_addrs::text ILIKE '%' || $5 || '%'
  ))`,
		`($6 = '' OR (
    cc_addrs::text ILIKE '%' || $6 || '%'
  ))`,
		`($7 = '' OR (
    bcc_addrs::text ILIKE '%' || $7 || '%'
  ))`,
		"($8 = '' OR subject ILIKE '%' || $8 || '%')",
	}
}

func messageSearchSQLWithConditions(sortMode string, conditions []string) string {
	orderBy := "message_at DESC, id DESC"
	if sortMode == MessageSearchSortRelevance {
		orderBy = "search_rank DESC NULLS LAST, message_at DESC, id DESC"
	}
	return fmt.Sprintf(`
WITH search_input AS (
  SELECT plainto_tsquery('simple', $2) AS tsq
),
ranked_messages AS (
SELECT
  messages.id::text AS id,
  messages.folder_id::text AS folder_id,
  messages.subject,
  left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview,
  messages.from_addr,
  messages.from_name,
  COALESCE(received_at, sent_at, draft_updated_at, created_at) AS message_at,
  messages.size,
  messages.has_attachment,
  COALESCE((flags->>'read')::boolean, false) AS read,
  COALESCE((flags->>'starred')::boolean, false) AS starred,
  CASE WHEN $11::boolean AND $2 <> '' THEN
    ts_rank_cd(
      setweight(to_tsvector('simple', coalesce(messages.subject, '')), 'A') ||
      setweight(to_tsvector('simple', coalesce(messages.from_addr, '')), 'A') ||
      setweight(to_tsvector('simple', coalesce(messages.from_name, '')), 'B') ||
      setweight(to_tsvector('simple', coalesce(msd.body_text, '')), 'D'),
      search_input.tsq
    )
  ELSE NULL END AS search_rank,
  CASE WHEN $12::boolean AND $2 <> '' THEN
    ts_headline('simple', coalesce(messages.subject, ''), search_input.tsq, 'StartSel=<mark>, StopSel=</mark>, MaxFragments=2, MinWords=1, MaxWords=12')
  ELSE NULL END AS subject_highlight,
  CASE WHEN $12::boolean AND $2 <> '' THEN
    ts_headline('simple', coalesce(messages.from_name, '') || ' ' || coalesce(messages.from_addr, ''), search_input.tsq, 'StartSel=<mark>, StopSel=</mark>, MaxFragments=2, MinWords=1, MaxWords=12')
  ELSE NULL END AS from_highlight,
  CASE WHEN $12::boolean AND $2 <> '' THEN
    ts_headline('simple', left(coalesce(msd.body_text, ''), 5000), search_input.tsq, 'StartSel=<mark>, StopSel=</mark>, MaxFragments=3, MinWords=3, MaxWords=18')
  ELSE NULL END AS body_highlight
FROM messages
CROSS JOIN search_input
LEFT JOIN message_search_documents msd
  ON msd.message_id = messages.id
 AND msd.user_id = messages.user_id
WHERE %s
)
SELECT
  id,
  folder_id,
  subject,
  preview,
  from_addr,
  from_name,
  message_at,
  size,
  has_attachment,
  read,
  starred,
  search_rank,
  subject_highlight,
  from_highlight,
  body_highlight
FROM ranked_messages
ORDER BY `+orderBy+`
LIMIT $10`, strings.Join(conditions, "\n  AND "))
}

func draftSearchSQL() string {
	return draftSearchSQLWithConditions(draftSearchBaseConditions())
}

func draftSearchBaseConditions() []string {
	return []string{
		"user_id = $1",
		"status = 'draft'",
		`($2 = '' OR (
    subject ILIKE '%' || $2 || '%'
    OR from_addr ILIKE '%' || $2 || '%'
    OR from_name ILIKE '%' || $2 || '%'
    OR to_addrs::text ILIKE '%' || $2 || '%'
    OR cc_addrs::text ILIKE '%' || $2 || '%'
    OR bcc_addrs::text ILIKE '%' || $2 || '%'
    OR draft_text_body ILIKE '%' || $2 || '%'
  ))`,
		`($3 = '' OR (
    from_addr ILIKE '%' || $3 || '%'
    OR from_name ILIKE '%' || $3 || '%'
  ))`,
		`($4 = '' OR (
    to_addrs::text ILIKE '%' || $4 || '%'
  ))`,
		`($5 = '' OR (
    cc_addrs::text ILIKE '%' || $5 || '%'
  ))`,
		`($6 = '' OR (
    bcc_addrs::text ILIKE '%' || $6 || '%'
  ))`,
		"($7 = '' OR subject ILIKE '%' || $7 || '%')",
	}
}

func draftSearchSQLWithConditions(conditions []string) string {
	return fmt.Sprintf(`
SELECT
  id::text,
  COALESCE(rfc_message_id, ''),
  subject,
  from_addr,
  from_name,
  to_addrs,
  cc_addrs,
  bcc_addrs,
  COALESCE(draft_updated_at, updated_at, created_at) AS draft_at,
  size,
  has_attachment,
  flags,
  storage_path,
  COALESCE(draft_text_body, '')
FROM messages
WHERE %s
ORDER BY draft_at DESC, id DESC
LIMIT $11`, strings.Join(conditions, "\n  AND "))
}

func buildDraftSearchSQL(hasAttachment, cursorID string) string {
	conditions := draftSearchBaseConditions()
	if hasAttachment != "" {
		conditions = append(conditions, "has_attachment = $8::boolean")
	}
	if cursorID != "" {
		conditions = append(conditions, "(COALESCE(draft_updated_at, updated_at, created_at), id) < ($9::timestamptz, $10::uuid)")
	}
	return draftSearchSQLWithConditions(conditions)
}

func searchHighlightsFromSQL(subject sql.NullString, from sql.NullString, body sql.NullString) *MessageSearchHighlights {
	highlights := MessageSearchHighlights{
		Subject: highlightFragments(subject),
		From:    highlightFragments(from),
		Body:    highlightFragments(body),
	}
	if len(highlights.Subject) == 0 && len(highlights.From) == 0 && len(highlights.Body) == 0 {
		return nil
	}
	return &highlights
}

func highlightFragments(value sql.NullString) []string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	if !strings.Contains(value.String, "<mark>") {
		return nil
	}
	return []string{value.String}
}
