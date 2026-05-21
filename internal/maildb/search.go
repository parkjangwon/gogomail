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
	sqlQuery := buildMessageSearchSQL(sortMode, query, hasAttachment, cursorID)
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

func buildMessageSearchSQL(sortMode string, query MessageSearchQuery, hasAttachment, cursorID string) string {
	conditions := messageSearchBaseConditions(query)
	folderID := strings.TrimSpace(query.FolderID)
	if folderID != "" {
		conditions = append(conditions, "messages.folder_id = $3::uuid")
	}
	if hasAttachment != "" {
		conditions = append(conditions, "messages.has_attachment = $9::boolean")
	}
	if cursorID != "" {
		conditions = append(conditions, "(COALESCE(messages.received_at, messages.sent_at, messages.draft_updated_at, messages.created_at), messages.id) < ($13::timestamptz, $14::uuid)")
	}
	return messageSearchSQLWithConditions(sortMode, conditions, messageSearchUsesQueryMatches(query))
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
		buildDraftSearchSQL(query, hasAttachment, cursorID),
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
	query := MessageSearchQuery{
		Query:   "quarterly",
		From:    "alice",
		To:      "bob",
		Cc:      "carol",
		Bcc:     "dave",
		Subject: "report",
	}
	return messageSearchSQLWithConditions(sortMode, messageSearchBaseConditions(query), messageSearchUsesQueryMatches(query))
}

func messageSearchBaseConditions(query MessageSearchQuery) []string {
	conditions := []string{
		"messages.user_id = $1",
		"messages.status = 'active'",
	}
	if strings.TrimSpace(query.Query) != "" {
		conditions = append(conditions, "messages.id IN (SELECT id FROM query_matches)")
	}
	if strings.TrimSpace(query.From) != "" {
		conditions = append(conditions, "messages.from_addr ILIKE '%' || $4 || '%'")
	}
	if strings.TrimSpace(query.To) != "" {
		conditions = append(conditions, "messages.to_addrs::text ILIKE '%' || $5 || '%'")
	}
	if strings.TrimSpace(query.Cc) != "" {
		conditions = append(conditions, "messages.cc_addrs::text ILIKE '%' || $6 || '%'")
	}
	if strings.TrimSpace(query.Bcc) != "" {
		conditions = append(conditions, "messages.bcc_addrs::text ILIKE '%' || $7 || '%'")
	}
	if strings.TrimSpace(query.Subject) != "" {
		conditions = append(conditions, "messages.subject ILIKE '%' || $8 || '%'")
	}
	return conditions
}

func messageSearchUsesQueryMatches(query MessageSearchQuery) bool {
	return strings.TrimSpace(query.Query) != ""
}

func messageSearchQueryMatchesCTE(include bool) string {
	if !include {
		return ""
	}
	return `,
query_matches AS (
  SELECT messages.id
  FROM messages
  CROSS JOIN search_input
  WHERE messages.user_id = $1
    AND messages.status = 'active'
    AND to_tsvector(
      'simple',
      coalesce(messages.subject, '') || ' ' ||
      coalesce(messages.from_addr, '') || ' ' ||
      coalesce(messages.from_name, '')
    ) @@ search_input.tsq
  UNION
  SELECT messages.id
  FROM messages
  WHERE messages.user_id = $1
    AND messages.status = 'active'
    AND messages.subject ILIKE '%%' || $2 || '%%'
  UNION
  SELECT messages.id
  FROM messages
  WHERE messages.user_id = $1
    AND messages.status = 'active'
    AND messages.from_addr ILIKE '%%' || $2 || '%%'
  UNION
  SELECT messages.id
  FROM messages
  WHERE messages.user_id = $1
    AND messages.status = 'active'
    AND messages.from_name ILIKE '%%' || $2 || '%%'
  UNION
  SELECT msd.message_id AS id
  FROM message_search_documents msd
  JOIN messages
    ON messages.id = msd.message_id
   AND messages.user_id = msd.user_id
  CROSS JOIN search_input
  WHERE msd.user_id = $1
    AND messages.status = 'active'
    AND to_tsvector('simple', msd.body_text) @@ search_input.tsq
  UNION
  SELECT msd.message_id AS id
  FROM message_search_documents msd
  JOIN messages
    ON messages.id = msd.message_id
   AND messages.user_id = msd.user_id
  WHERE msd.user_id = $1
    AND messages.status = 'active'
    AND msd.body_text ILIKE '%%' || $2 || '%%'
)`
}

func messageSearchSQLWithConditions(sortMode string, conditions []string, includeQueryMatches bool) string {
	orderBy := "message_at DESC, id DESC"
	if sortMode == MessageSearchSortRelevance {
		orderBy = "search_rank DESC NULLS LAST, message_at DESC, id DESC"
	}
	queryMatchesCTE := messageSearchQueryMatchesCTE(includeQueryMatches)
	return fmt.Sprintf(`
WITH search_input AS (
  SELECT plainto_tsquery('simple', $2) AS tsq
)`+queryMatchesCTE+`,
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
	query := DraftSearchQuery{
		Query:   "quarterly",
		From:    "alice",
		To:      "bob",
		Cc:      "carol",
		Bcc:     "dave",
		Subject: "report",
	}
	return draftSearchSQLWithConditions(draftSearchBaseConditions(query), draftSearchUsesQueryMatches(query))
}

func draftSearchBaseConditions(query DraftSearchQuery) []string {
	conditions := []string{
		"user_id = $1",
		"status = 'draft'",
	}
	if strings.TrimSpace(query.Query) != "" {
		conditions = append(conditions, "id IN (SELECT id FROM draft_matches)")
	}
	if strings.TrimSpace(query.From) != "" {
		conditions = append(conditions, `(
    from_addr ILIKE '%' || $3 || '%'
    OR from_name ILIKE '%' || $3 || '%'
  )`)
	}
	if strings.TrimSpace(query.To) != "" {
		conditions = append(conditions, "to_addrs::text ILIKE '%' || $4 || '%'")
	}
	if strings.TrimSpace(query.Cc) != "" {
		conditions = append(conditions, "cc_addrs::text ILIKE '%' || $5 || '%'")
	}
	if strings.TrimSpace(query.Bcc) != "" {
		conditions = append(conditions, "bcc_addrs::text ILIKE '%' || $6 || '%'")
	}
	if strings.TrimSpace(query.Subject) != "" {
		conditions = append(conditions, "subject ILIKE '%' || $7 || '%'")
	}
	return conditions
}

func draftSearchUsesQueryMatches(query DraftSearchQuery) bool {
	return strings.TrimSpace(query.Query) != ""
}

func draftSearchQueryMatchesCTE(include bool) string {
	if !include {
		return ""
	}
	return `
WITH draft_matches AS (
  SELECT id
  FROM messages
  WHERE user_id = $1
    AND status = 'draft'
    AND subject ILIKE '%%' || $2 || '%%'
  UNION
  SELECT id
  FROM messages
  WHERE user_id = $1
    AND status = 'draft'
    AND from_addr ILIKE '%%' || $2 || '%%'
  UNION
  SELECT id
  FROM messages
  WHERE user_id = $1
    AND status = 'draft'
    AND from_name ILIKE '%%' || $2 || '%%'
  UNION
  SELECT id
  FROM messages
  WHERE user_id = $1
    AND status = 'draft'
    AND to_addrs::text ILIKE '%%' || $2 || '%%'
  UNION
  SELECT id
  FROM messages
  WHERE user_id = $1
    AND status = 'draft'
    AND cc_addrs::text ILIKE '%%' || $2 || '%%'
  UNION
  SELECT id
  FROM messages
  WHERE user_id = $1
    AND status = 'draft'
    AND bcc_addrs::text ILIKE '%%' || $2 || '%%'
  UNION
  SELECT id
  FROM messages
  WHERE user_id = $1
    AND status = 'draft'
    AND draft_text_body ILIKE '%%' || $2 || '%%'
)
`
}

func draftSearchSQLWithConditions(conditions []string, includeQueryMatches bool) string {
	queryMatchesCTE := draftSearchQueryMatchesCTE(includeQueryMatches)
	return fmt.Sprintf(`
`+queryMatchesCTE+`
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

func buildDraftSearchSQL(query DraftSearchQuery, hasAttachment, cursorID string) string {
	conditions := draftSearchBaseConditions(query)
	if hasAttachment != "" {
		conditions = append(conditions, "has_attachment = $8::boolean")
	}
	if cursorID != "" {
		conditions = append(conditions, "(COALESCE(draft_updated_at, updated_at, created_at), id) < ($9::timestamptz, $10::uuid)")
	}
	return draftSearchSQLWithConditions(conditions, draftSearchUsesQueryMatches(query))
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
