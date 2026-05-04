package maildb

import (
	"context"
	"fmt"
	"strings"
)

type MessageSearchDocument struct {
	MessageID         string
	UserID            string
	BodyText          string
	BodyTextTruncated bool
}

func (r *Repository) UpsertMessageSearchDocument(ctx context.Context, doc MessageSearchDocument) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	messageID := strings.TrimSpace(doc.MessageID)
	if messageID == "" {
		return fmt.Errorf("message_id is required")
	}
	userID := strings.TrimSpace(doc.UserID)
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}

	const query = `
INSERT INTO message_search_documents (
  message_id,
  user_id,
  body_text,
  body_text_truncated,
  indexed_at
) VALUES ($1, $2, $3, $4, now())
ON CONFLICT (message_id) DO UPDATE
SET user_id = EXCLUDED.user_id,
    body_text = EXCLUDED.body_text,
    body_text_truncated = EXCLUDED.body_text_truncated,
    indexed_at = now()`

	if _, err := r.db.ExecContext(ctx, query, messageID, userID, doc.BodyText, doc.BodyTextTruncated); err != nil {
		return fmt.Errorf("upsert message search document: %w", err)
	}
	return nil
}
