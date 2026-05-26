package jmap

import (
	"context"
	"encoding/json"
)

// emailChangesMethod implements Email/changes (RFC 8621 §4.2).
type emailChangesMethod struct{ deps Deps }

func (m *emailChangesMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return errorResult(ErrServerFail), nil
	}
	var req ChangesArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResult(ErrInvalidArguments), nil
	}
	if req.AccountID == "" {
		req.AccountID = userID
	}

	sinceModSeq := ParseModSeqState(req.SinceState)
	newState, _ := EmailStateFor(ctx, m.deps.Repo.DB(), userID)

	resp := ChangesResponse{
		AccountID:      req.AccountID,
		OldState:       req.SinceState,
		NewState:       newState,
		HasMoreChanges: false,
		Created:        []string{},
		Updated:        []string{},
		Destroyed:      []string{},
	}

	// If state hasn't changed, return empty response.
	if req.SinceState == newState {
		return json.Marshal(resp)
	}

	// Query messages with modseq > sinceModSeq — these were created or updated.
	limit := req.MaxChanges
	if limit <= 0 || limit > maxObjectsInGet {
		limit = maxObjectsInGet
	}

	rows, err := m.deps.Repo.DB().QueryContext(ctx, `
		SELECT i.message_id::text, COALESCE(m.status, '')
		FROM imap_message_uid i
		LEFT JOIN messages m ON m.id = i.message_id AND m.user_id = $1::uuid
		WHERE i.user_id = $1::uuid
		  AND i.modseq > $2
		ORDER BY i.modseq ASC
		LIMIT $3`,
		userID, sinceModSeq, limit+1,
	)
	if err != nil {
		return errorResult(ErrServerFail), nil
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		if count >= limit {
			resp.HasMoreChanges = true
			break
		}
		var id, status string
		if err := rows.Scan(&id, &status); err != nil {
			continue
		}
		if status == "active" {
			// Conservatively mark all active changes as updated;
			// distinguishing created vs. updated would require tracking
			// the first-seen modseq, which is not stored separately.
			resp.Updated = append(resp.Updated, id)
		} else {
			// Empty/null status means the message row is gone or deleted.
			resp.Destroyed = append(resp.Destroyed, id)
		}
		count++
	}

	return json.Marshal(resp)
}
