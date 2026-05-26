package jmap

import (
	"context"
	"encoding/json"

	"github.com/gogomail/gogomail/internal/maildb"
)

// emailQueryMethod implements Email/query (RFC 8621 §4.4).
type emailQueryMethod struct{ deps Deps }

func (m *emailQueryMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return errorResult(ErrServerFail), nil
	}
	var req EmailQueryArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResult(ErrInvalidArguments), nil
	}
	if req.AccountID == "" {
		req.AccountID = userID
	}

	limit := req.Limit
	if limit <= 0 || limit > maxObjectsInGet {
		limit = maxObjectsInGet
	}

	state, _ := EmailStateFor(ctx, m.deps.Repo.DB(), userID)

	var msgs []maildb.MessageSummary
	var err error

	folderID := ""
	if req.Filter != nil {
		folderID = req.Filter.InMailbox
	}

	// Choose query path: text search vs. paginated list.
	if req.Filter != nil && req.Filter.Text != "" {
		searchQuery := maildb.MessageSearchQuery{
			UserID:   userID,
			Query:    req.Filter.Text,
			FolderID: folderID,
			From:     req.Filter.From,
			To:       req.Filter.To,
			Subject:  req.Filter.Subject,
			Limit:    limit,
			Sort:     "date",
		}
		msgs, err = m.deps.Repo.SearchMessages(ctx, searchQuery)
	} else {
		filter := maildb.MessageListFilter{}
		if req.Filter != nil {
			if req.Filter.HasKeyword == "$seen" {
				t := true
				filter.Read = &t
			}
			if req.Filter.NotKeyword == "$seen" {
				f := false
				filter.Read = &f
			}
			if req.Filter.HasKeyword == "$flagged" {
				t := true
				filter.Starred = &t
			}
		}
		// Map sort comparator.
		if len(req.Sort) > 0 && req.Sort[0].IsAscending {
			filter.Sort = "oldest"
		} else {
			filter.Sort = "newest"
		}
		msgs, err = m.deps.Repo.ListMessagesPage(ctx, userID, folderID, limit, maildb.MessageListCursor{}, filter)
	}
	if err != nil {
		return errorResult(ErrServerFail), nil
	}

	ids := make([]string, len(msgs))
	for i, msg := range msgs {
		ids[i] = msg.ID
	}

	// Apply position offset (JMAP position is 0-based index into the full result).
	pos := req.Position
	if pos < 0 {
		pos = 0
	}
	if pos > len(ids) {
		pos = len(ids)
	}
	ids = ids[pos:]

	return json.Marshal(EmailQueryResponse{
		AccountID:           req.AccountID,
		QueryState:          state,
		CanCalculateChanges: false,
		Position:            req.Position,
		IDs:                 ids,
		Total:               len(ids) + pos, // approximate: total of this page
	})
}

// emailQueryChangesMethod implements Email/queryChanges (RFC 8621 §4.4.2).
// Returns cannotCalculateChanges — the server does not track query result history.
type emailQueryChangesMethod struct{ deps Deps }

func (m *emailQueryChangesMethod) Call(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
	return errorResult("cannotCalculateChanges"), nil
}
