package jmap

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// DraftSender abstracts email submission for EmailSubmission/set.
// mailservice.Service satisfies this interface via an adapter.
type DraftSender interface {
	SendDraft(ctx context.Context, userID, draftID string) error
}

type emailSubmissionCreate struct {
	EmailID    string          `json:"emailId"`
	IdentityID string          `json:"identityId"`
	Envelope   json.RawMessage `json:"envelope,omitempty"`
}

type emailSubmissionSetArgs struct {
	AccountID string                            `json:"accountId"`
	Create    map[string]emailSubmissionCreate  `json:"create,omitempty"`
	Update    map[string]json.RawMessage        `json:"update,omitempty"`
	Destroy   []string                          `json:"destroy,omitempty"`
}

type emailSubmissionResponse struct {
	AccountID    string                     `json:"accountId"`
	OldState     string                     `json:"oldState"`
	NewState     string                     `json:"newState"`
	Created      map[string]json.RawMessage `json:"created"`
	NotCreated   map[string]SetError        `json:"notCreated"`
	Updated      map[string]json.RawMessage `json:"updated"`
	NotUpdated   map[string]SetError        `json:"notUpdated"`
	Destroyed    []string                   `json:"destroyed"`
	NotDestroyed map[string]SetError        `json:"notDestroyed"`
}

// emailSubmissionSetMethod implements EmailSubmission/set (RFC 8621 §7.5).
type emailSubmissionSetMethod struct{ deps Deps }

func (m *emailSubmissionSetMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return nil, fmt.Errorf("serverFail")
	}
	var req emailSubmissionSetArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}

	created := make(map[string]json.RawMessage)
	notCreated := make(map[string]SetError)

	for cid, sub := range req.Create {
		if m.deps.Sender == nil {
			notCreated[cid] = SetError{Type: "serverFail", Description: "email submission not available"}
			continue
		}
		if sub.EmailID == "" {
			notCreated[cid] = SetError{Type: "invalidProperties", Description: "emailId is required"}
			continue
		}
		if sub.IdentityID == "" {
			notCreated[cid] = SetError{Type: "invalidProperties", Description: "identityId is required"}
			continue
		}
		// Verify the draft exists.
		if _, err := m.deps.Repo.GetMessage(ctx, userID, sub.EmailID); err != nil {
			notCreated[cid] = SetError{Type: "notFound", Description: "email not found"}
			continue
		}
		// Send it.
		if err := m.deps.Sender.SendDraft(ctx, userID, sub.EmailID); err != nil {
			notCreated[cid] = SetError{Type: "serverFail", Description: err.Error()}
			continue
		}
		threadID := ""
		mailboxIDs := map[string]bool{}

		sendAt := time.Now().UTC().Format(time.RFC3339)

		// Return the submission record.
		submission := map[string]interface{}{
			"id":             sub.EmailID + "-submitted",
			"emailId":        sub.EmailID,
			"identityId":     sub.IdentityID,
			"threadId":       threadID,
			"mailboxIds":     mailboxIDs,
			"envelope":       nil,
			"sendAt":         sendAt,
			"undoStatus":     "final",
			"deliveryStatus": map[string]interface{}{},
			"dsnBlobIds":     []string{},
			"mdnBlobIds":     []string{},
		}
		raw, err := json.Marshal(submission)
		if err != nil {
			notCreated[cid] = SetError{Type: "serverFail", Description: "failed to encode submission"}
			continue
		}
		created[cid] = raw
	}

	notUpdated := make(map[string]SetError)
	for uid := range req.Update {
		notUpdated[uid] = SetError{Type: "forbidden"}
	}
	notDestroyed := make(map[string]SetError)
	for _, did := range req.Destroy {
		notDestroyed[did] = SetError{Type: "forbidden"}
	}

	oldState := "submission-v1"
	newState := oldState
	if len(created) > 0 {
		newState = fmt.Sprintf("submission-%d", time.Now().UnixMicro())
	}

	resp := emailSubmissionResponse{
		AccountID:    userID,
		OldState:     oldState,
		NewState:     newState,
		Created:      created,
		NotCreated:   notCreated,
		Updated:      make(map[string]json.RawMessage),
		NotUpdated:   notUpdated,
		Destroyed:    []string{},
		NotDestroyed: notDestroyed,
	}
	return json.Marshal(resp)
}
