package jmap

import (
	"context"
	"encoding/json"

	"github.com/gogomail/gogomail/internal/maildb"
)

// Thread object per RFC 8621 §4.
type Thread struct {
	ID       string   `json:"id"`
	EmailIDs []string `json:"emailIds"`
}

type ThreadGetArgs struct {
	AccountID  string   `json:"accountId"`
	IDs        []string `json:"ids"`
	Properties []string `json:"properties,omitempty"`
}

type ThreadGetResponse struct {
	AccountID string   `json:"accountId"`
	List      []Thread `json:"list"`
	NotFound  []string `json:"notFound"`
	State     string   `json:"state"`
}

// threadGetMethod implements Thread/get (RFC 8621 §4).
type threadGetMethod struct{ deps Deps }

func (m *threadGetMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return errorResult(ErrServerFail), nil
	}
	var req ThreadGetArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResult(ErrInvalidArguments), nil
	}
	if req.AccountID == "" {
		req.AccountID = userID
	}

	state, _ := EmailStateFor(ctx, m.deps.Repo.DB(), userID)
	resp := ThreadGetResponse{
		AccountID: req.AccountID,
		State:     state,
		List:      make([]Thread, 0),
		NotFound:  make([]string, 0),
	}

	for _, threadID := range req.IDs {
		msgs, err := m.deps.Repo.ListThreadMessagesPage(ctx, userID, threadID, 500, maildb.MessageListCursor{})
		if err != nil || len(msgs) == 0 {
			resp.NotFound = append(resp.NotFound, threadID)
			continue
		}
		emailIDs := make([]string, len(msgs))
		for i, msg := range msgs {
			emailIDs[i] = msg.ID
		}
		resp.List = append(resp.List, Thread{ID: threadID, EmailIDs: emailIDs})
	}
	return json.Marshal(resp)
}

// threadChangesMethod implements Thread/changes (RFC 8621 §4).
// Thread state tracks with email state (modseq).
type threadChangesMethod struct{ deps Deps }

func (m *threadChangesMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
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
	return json.Marshal(resp)
}
