package jmap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/outbound"
)

// EmailSetArgs is the argument object for Email/set (RFC 8621 §4.6).
type EmailSetArgs struct {
	AccountID string                      `json:"accountId"`
	Create    map[string]EmailCreatePatch `json:"create,omitempty"`
	Update    map[string]json.RawMessage  `json:"update,omitempty"`
	Destroy   []string                    `json:"destroy,omitempty"`
}

// EmailCreatePatch is the Email object used when creating a new draft.
type EmailCreatePatch struct {
	MailboxIDs map[string]bool          `json:"mailboxIds"`
	Keywords   map[string]bool          `json:"keywords,omitempty"`
	Subject    string                   `json:"subject,omitempty"`
	From       []EmailAddress           `json:"from,omitempty"`
	To         []EmailAddress           `json:"to,omitempty"`
	Cc         []EmailAddress           `json:"cc,omitempty"`
	Bcc        []EmailAddress           `json:"bcc,omitempty"`
	BodyValues map[string]EmailBodyValue `json:"bodyValues,omitempty"`
	TextBody   []EmailBodyPart          `json:"textBody,omitempty"`
	HTMLBody   []EmailBodyPart          `json:"htmlBody,omitempty"`
}

// emailSetMethod implements Email/set (RFC 8621 §4.6).
type emailSetMethod struct{ deps Deps }

func (m *emailSetMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	var req EmailSetArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResult(ErrInvalidArguments), nil
	}
	if m.deps.Repo == nil {
		return errorResult(ErrServerFail), nil
	}
	if req.AccountID == "" {
		req.AccountID = userID
	}

	oldState, _ := EmailStateFor(ctx, m.deps.Repo.DB(), userID)
	resp := SetResponse{
		AccountID:    req.AccountID,
		OldState:     oldState,
		Created:      make(map[string]any),
		Updated:      make(map[string]any),
		NotCreated:   make(map[string]SetError),
		NotUpdated:   make(map[string]SetError),
		NotDestroyed: make(map[string]SetError),
	}

	// Destroy
	for _, id := range req.Destroy {
		if err := m.deps.Repo.DeleteMessage(ctx, userID, id); err != nil {
			resp.NotDestroyed[id] = SetError{Type: "notFound", Description: err.Error()}
			continue
		}
		resp.Destroyed = append(resp.Destroyed, id)
	}

	// Update (patch semantics)
	for id, patch := range req.Update {
		if err := applyEmailPatch(ctx, m.deps.Repo, userID, id, patch); err != nil {
			resp.NotUpdated[id] = SetError{Type: "notFound", Description: err.Error()}
			continue
		}
		resp.Updated[id] = nil
	}

	// Create (draft)
	for clientID, create := range req.Create {
		msgID, err := createDraftEmail(ctx, m.deps.Repo, userID, create)
		if err != nil {
			resp.NotCreated[clientID] = SetError{Type: "invalidProperties", Description: err.Error()}
			continue
		}
		resp.Created[clientID] = map[string]any{"id": msgID}
	}

	resp.NewState, _ = EmailStateFor(ctx, m.deps.Repo.DB(), userID)
	return json.Marshal(resp)
}

// applyEmailPatch applies a JSON Patch-style update to an email.
// Supports: keywords/$seen, keywords/$flagged, keywords/$draft, mailboxIds/{folderId}
func applyEmailPatch(ctx context.Context, repo *maildb.Repository, userID, msgID string, patch json.RawMessage) error {
	var p map[string]any
	if err := json.Unmarshal(patch, &p); err != nil {
		return fmt.Errorf("Email/set: unmarshal patch: %w", err)
	}
	for key, val := range p {
		switch {
		case key == "keywords/$seen":
			if err := repo.SetMessageFlag(ctx, userID, msgID, "read", asBool(val)); err != nil {
				return fmt.Errorf("Email/set: set $seen flag: %w", err)
			}
		case key == "keywords/$flagged":
			if err := repo.SetMessageFlag(ctx, userID, msgID, "starred", asBool(val)); err != nil {
				return fmt.Errorf("Email/set: set $flagged flag: %w", err)
			}
		case key == "keywords/$draft":
			if err := repo.SetMessageFlag(ctx, userID, msgID, "draft", asBool(val)); err != nil {
				return fmt.Errorf("Email/set: set $draft flag: %w", err)
			}
		case strings.HasPrefix(key, "mailboxIds/"):
			folderID := strings.TrimPrefix(key, "mailboxIds/")
			if asBool(val) {
				if err := repo.MoveMessage(ctx, userID, msgID, folderID); err != nil {
					return fmt.Errorf("Email/set: move to mailbox: %w", err)
				}
			}
		}
	}
	return nil
}

func asBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// createDraftEmail creates a new draft email in the user's Drafts folder.
func createDraftEmail(ctx context.Context, repo *maildb.Repository, userID string, e EmailCreatePatch) (string, error) {
	// Extract body text from bodyValues
	textBody := ""
	htmlBody := ""
	if len(e.TextBody) > 0 && e.BodyValues != nil {
		if bv, ok := e.BodyValues[e.TextBody[0].PartID]; ok {
			textBody = bv.Value
		}
	}
	if len(e.HTMLBody) > 0 && e.BodyValues != nil {
		if bv, ok := e.BodyValues[e.HTMLBody[0].PartID]; ok {
			htmlBody = bv.Value
		}
	}

	fromAddr := ""
	if len(e.From) > 0 {
		fromAddr = e.From[0].Email
	}

	req := maildb.SaveDraftRequest{
		UserID:   userID,
		From:     fromAddr,
		Subject:  e.Subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}
	for _, a := range e.To {
		name := ""
		if a.Name != nil {
			name = *a.Name
		}
		req.To = append(req.To, outbound.Address{Name: name, Email: a.Email})
	}
	for _, a := range e.Cc {
		name := ""
		if a.Name != nil {
			name = *a.Name
		}
		req.Cc = append(req.Cc, outbound.Address{Name: name, Email: a.Email})
	}
	for _, a := range e.Bcc {
		name := ""
		if a.Name != nil {
			name = *a.Name
		}
		req.Bcc = append(req.Bcc, outbound.Address{Name: name, Email: a.Email})
	}

	detail, err := repo.SaveDraft(ctx, req)
	if err != nil {
		return "", err
	}
	return detail.ID, nil
}
