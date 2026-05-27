package maildb

import (
	"errors"
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type SourceThreadView struct {
	MessageID string
	InReplyTo string
	ThreadID  string
}

func (v SourceThreadView) References() []string {
	refs := make([]string, 0, 2)
	if strings.TrimSpace(v.InReplyTo) != "" {
		refs = append(refs, v.InReplyTo)
	}
	if strings.TrimSpace(v.MessageID) != "" {
		refs = append(refs, v.MessageID)
	}
	return normalizeThreadCandidates(refs)
}

func (r *Repository) SourceThread(ctx context.Context, userID string, sourceMessageID string) (SourceThreadView, error) {
	if r.db == nil {
		return SourceThreadView{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	sourceMessageID = strings.TrimSpace(sourceMessageID)
	if userID == "" {
		return SourceThreadView{}, fmt.Errorf("user_id is required")
	}
	if sourceMessageID == "" {
		return SourceThreadView{}, fmt.Errorf("source message id is required")
	}

	const query = `
SELECT
  COALESCE(rfc_message_id, ''),
  COALESCE(in_reply_to, ''),
  COALESCE(thread_id, id)::text
FROM messages
WHERE user_id = $1
  AND id = $2
  AND status = 'active'
LIMIT 1`

	var source SourceThreadView
	if err := r.db.QueryRowContext(ctx, query, userID, sourceMessageID).Scan(&source.MessageID, &source.InReplyTo, &source.ThreadID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SourceThreadView{}, fmt.Errorf("source message %q not found", sourceMessageID)
		}
		return SourceThreadView{}, fmt.Errorf("read source thread: %w", err)
	}
	return source, nil
}
