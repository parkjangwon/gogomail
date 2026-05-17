package maildb

import (
	"context"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

const listMessagesByIDsSQL = `
WITH requested AS (
  SELECT value AS id, ordinality
  FROM unnest($2::uuid[]) WITH ORDINALITY AS requested(value, ordinality)
)
SELECT
  m.id::text,
  m.folder_id::text,
  m.subject,
  left(btrim(regexp_replace(left(coalesce(msd.body_text, ''), 2000), '[[:space:]]+', ' ', 'g')), 280) AS preview,
  m.from_addr,
  m.from_name,
  COALESCE(m.received_at, m.sent_at, m.draft_updated_at, m.created_at) AS message_at,
  m.size,
  m.has_attachment,
  COALESCE((m.flags->>'read')::boolean, false) AS read,
  COALESCE((m.flags->>'starred')::boolean, false) AS starred
FROM requested
JOIN messages m ON m.id = requested.id
LEFT JOIN message_search_documents msd
  ON msd.message_id = m.id
 AND msd.user_id = m.user_id
WHERE m.user_id = $1::uuid
  AND m.status = 'active'
ORDER BY requested.ordinality`

func (r *Repository) ListMessagesByIDs(ctx context.Context, userID string, messageIDs []string) ([]MessageSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	messageIDs, err := normalizeSearchMessageIDs(messageIDs)
	if err != nil {
		return nil, err
	}
	if len(messageIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, listMessagesByIDsSQL, userID, pq.Array(messageIDs))
	if err != nil {
		return nil, fmt.Errorf("list messages by ids: %w", err)
	}
	defer rows.Close()

	messages := make([]MessageSummary, 0, len(messageIDs))
	for rows.Next() {
		var msg MessageSummary
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
		); err != nil {
			return nil, fmt.Errorf("scan hydrated search message: %w", err)
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate hydrated search messages: %w", err)
	}
	return messages, nil
}

func normalizeSearchMessageIDs(messageIDs []string) ([]string, error) {
	if len(messageIDs) > MessageListMaxLimit {
		return nil, fmt.Errorf("too many message ids")
	}
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
	return out, nil
}
