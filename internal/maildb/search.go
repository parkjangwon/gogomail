package maildb

import (
	"context"
	"fmt"
	"strings"
)

type MessageSearchQuery struct {
	UserID        string
	Query         string
	FolderID      string
	From          string
	Subject       string
	HasAttachment *bool
	Limit         int
}

func (q MessageSearchQuery) normalizedLimit() int {
	return normalizeLimit(q.Limit)
}

func (r *Repository) SearchMessages(ctx context.Context, query MessageSearchQuery) ([]MessageSummary, error) {
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

	const sql = `
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
  AND ($2 = '' OR (
    to_tsvector(
      'simple',
      coalesce(subject, '') || ' ' ||
      coalesce(from_addr, '') || ' ' ||
      coalesce(from_name, '') || ' ' ||
      coalesce(draft_text_body, '')
    ) @@ plainto_tsquery('simple', $2)
    OR subject ILIKE '%' || $2 || '%'
    OR from_addr ILIKE '%' || $2 || '%'
  ))
  AND ($3 = '' OR folder_id::text = $3)
  AND ($4 = '' OR from_addr ILIKE '%' || $4 || '%')
  AND ($5 = '' OR subject ILIKE '%' || $5 || '%')
  AND ($6 = '' OR has_attachment = $6::boolean)
ORDER BY message_at DESC, id DESC
LIMIT $7`

	rows, err := r.db.QueryContext(
		ctx,
		sql,
		userID,
		strings.TrimSpace(query.Query),
		strings.TrimSpace(query.FolderID),
		strings.TrimSpace(query.From),
		strings.TrimSpace(query.Subject),
		hasAttachment,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()

	messages := make([]MessageSummary, 0, limit)
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
			return nil, fmt.Errorf("scan search message: %w", err)
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search messages: %w", err)
	}
	return messages, nil
}
