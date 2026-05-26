package jmap

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogomail/gogomail/internal/maildb"
)

// Mailbox represents a JMAP Mailbox object (RFC 8621 §2).
type Mailbox struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	ParentID      *string       `json:"parentId"`
	Role          *string       `json:"role"`
	SortOrder     int           `json:"sortOrder"`
	TotalEmails   int64         `json:"totalEmails"`
	UnreadEmails  int64         `json:"unreadEmails"`
	TotalThreads  int64         `json:"totalThreads"`
	UnreadThreads int64         `json:"unreadThreads"`
	MyRights      MailboxRights `json:"myRights"`
	IsSubscribed  bool          `json:"isSubscribed"`
}

// MailboxRights represents the per-mailbox rights for the authenticated user (RFC 8621 §2).
type MailboxRights struct {
	MayReadItems   bool `json:"mayReadItems"`
	MayAddItems    bool `json:"mayAddItems"`
	MayRemoveItems bool `json:"mayRemoveItems"`
	MaySetSeen     bool `json:"maySetSeen"`
	MaySetKeywords bool `json:"maySetKeywords"`
	MayCreateChild bool `json:"mayCreateChild"`
	MayRename      bool `json:"mayRename"`
	MayDelete      bool `json:"mayDelete"`
	MaySubmit      bool `json:"maySubmit"`
}

// MailboxGetArgs is the argument object for Mailbox/get.
type MailboxGetArgs struct {
	AccountID  string   `json:"accountId"`
	IDs        []string `json:"ids"` // null / omitted = all mailboxes
	Properties []string `json:"properties,omitempty"`
}

// MailboxGetResponse is the response object for Mailbox/get.
type MailboxGetResponse struct {
	AccountID string    `json:"accountId"`
	List      []Mailbox `json:"list"`
	NotFound  []string  `json:"notFound"`
	State     string    `json:"state"`
}

// MailboxQueryArgs is the argument object for Mailbox/query.
type MailboxQueryArgs struct {
	AccountID string `json:"accountId"`
	Position  int    `json:"position,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// MailboxQueryResponse is the response object for Mailbox/query.
type MailboxQueryResponse struct {
	AccountID           string   `json:"accountId"`
	QueryState          string   `json:"queryState"`
	CanCalculateChanges bool     `json:"canCalculateChanges"`
	Position            int      `json:"position"`
	IDs                 []string `json:"ids"`
	Total               int      `json:"total"`
}

// MailboxSetArgs is the argument object for Mailbox/set.
type MailboxSetArgs struct {
	AccountID string                  `json:"accountId"`
	Create    map[string]MailboxPatch `json:"create,omitempty"`
	Update    map[string]MailboxPatch `json:"update,omitempty"`
	Destroy   []string                `json:"destroy,omitempty"`
}

// MailboxPatch carries the properties for a Mailbox create/update.
type MailboxPatch struct {
	Name     string  `json:"name,omitempty"`
	ParentID *string `json:"parentId,omitempty"`
}

// SetResponse is the response object for any JMAP /set method.
type SetResponse struct {
	AccountID    string              `json:"accountId"`
	OldState     string              `json:"oldState"`
	NewState     string              `json:"newState"`
	Created      map[string]any      `json:"created,omitempty"`
	Updated      map[string]any      `json:"updated,omitempty"`
	Destroyed    []string            `json:"destroyed,omitempty"`
	NotCreated   map[string]SetError `json:"notCreated,omitempty"`
	NotUpdated   map[string]SetError `json:"notUpdated,omitempty"`
	NotDestroyed map[string]SetError `json:"notDestroyed,omitempty"`
}

// SetError is a per-object error returned inside notCreated / notUpdated / notDestroyed.
type SetError struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// ChangesArgs is the argument object for Mailbox/changes.
type ChangesArgs struct {
	AccountID  string `json:"accountId"`
	SinceState string `json:"sinceState"`
	MaxChanges int    `json:"maxChanges,omitempty"`
}

// ChangesResponse is the response object for Mailbox/changes.
type ChangesResponse struct {
	AccountID      string   `json:"accountId"`
	OldState       string   `json:"oldState"`
	NewState       string   `json:"newState"`
	HasMoreChanges bool     `json:"hasMoreChanges"`
	Created        []string `json:"created"`
	Updated        []string `json:"updated"`
	Destroyed      []string `json:"destroyed"`
}

// folderRole maps gogomail system_type values to JMAP role strings (RFC 8621 §2).
var folderRole = map[string]string{
	"inbox":   "inbox",
	"sent":    "sent",
	"drafts":  "drafts",
	"trash":   "trash",
	"spam":    "junk",
	"archive": "archive",
}

// folderToMailbox converts a maildb.Folder into a JMAP Mailbox object.
func folderToMailbox(f maildb.Folder) Mailbox {
	mb := Mailbox{
		ID:            f.ID,
		Name:          f.Name,
		SortOrder:     f.OrderIndex,
		TotalEmails:   f.Total,
		UnreadEmails:  f.Unread,
		TotalThreads:  f.Total,
		UnreadThreads: f.Unread,
		IsSubscribed:  true,
		MyRights: MailboxRights{
			MayReadItems:   true,
			MayAddItems:    true,
			MayRemoveItems: true,
			MaySetSeen:     true,
			MaySetKeywords: true,
			MayCreateChild: true,
			MayRename:      true,
			// System folders cannot be deleted; user folders can.
			MayDelete: f.SystemType == "",
			// Sent or user-created folders allow submission.
			MaySubmit: f.SystemType == "sent" || f.SystemType == "",
		},
	}
	if f.ParentID != "" {
		p := f.ParentID
		mb.ParentID = &p
	}
	if role, ok := folderRole[f.SystemType]; ok {
		r := role
		mb.Role = &r
	}
	return mb
}

// mailboxGetMethod implements the Mailbox/get JMAP method (RFC 8621 §2).
type mailboxGetMethod struct{ deps Deps }

func (m *mailboxGetMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return errorResult(ErrServerFail), nil
	}

	var req MailboxGetArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResult(ErrInvalidArguments), nil
	}
	if req.AccountID == "" {
		req.AccountID = userID
	}

	folders, err := m.deps.Repo.ListFolders(ctx, userID)
	if err != nil {
		return errorResult(ErrServerFail), nil
	}
	state, _ := MailboxStateFor(ctx, m.deps.Repo.DB(), userID)

	resp := MailboxGetResponse{
		AccountID: req.AccountID,
		State:     state,
		List:      make([]Mailbox, 0),
		NotFound:  make([]string, 0),
	}

	// Build a set of requested IDs for O(1) lookup. An empty IDs slice means
	// "return all" (per RFC 8620 §5.1: ids == null).
	idSet := make(map[string]bool, len(req.IDs))
	for _, id := range req.IDs {
		idSet[id] = true
	}
	foundIDs := make(map[string]bool, len(req.IDs))
	for _, f := range folders {
		if len(req.IDs) > 0 && !idSet[f.ID] {
			continue
		}
		resp.List = append(resp.List, folderToMailbox(f))
		foundIDs[f.ID] = true
	}
	for _, id := range req.IDs {
		if !foundIDs[id] {
			resp.NotFound = append(resp.NotFound, id)
		}
	}
	return json.Marshal(resp)
}

// mailboxQueryMethod implements the Mailbox/query JMAP method (RFC 8621 §2).
type mailboxQueryMethod struct{ deps Deps }

func (m *mailboxQueryMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return errorResult(ErrServerFail), nil
	}

	var req MailboxQueryArgs
	// best-effort decode; a missing/empty body is valid
	_ = json.Unmarshal(args, &req)
	if req.AccountID == "" {
		req.AccountID = userID
	}

	folders, err := m.deps.Repo.ListFolders(ctx, userID)
	if err != nil {
		return errorResult(ErrServerFail), nil
	}
	state, _ := MailboxStateFor(ctx, m.deps.Repo.DB(), userID)

	allIDs := make([]string, 0, len(folders))
	for _, f := range folders {
		allIDs = append(allIDs, f.ID)
	}

	const maxGet = 500
	limit := req.Limit
	if limit <= 0 || limit > maxGet {
		limit = maxGet
	}
	pos := req.Position
	if pos > len(allIDs) {
		pos = len(allIDs)
	}
	pageIDs := allIDs[pos:]
	if len(pageIDs) > limit {
		pageIDs = pageIDs[:limit]
	}

	return json.Marshal(MailboxQueryResponse{
		AccountID:           req.AccountID,
		QueryState:          state,
		CanCalculateChanges: false,
		Position:            pos,
		IDs:                 pageIDs,
		Total:               len(allIDs),
	})
}

// mailboxSetMethod implements the Mailbox/set JMAP method (RFC 8621 §2).
type mailboxSetMethod struct{ deps Deps }

func (m *mailboxSetMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
	if m.deps.Repo == nil {
		return errorResult(ErrServerFail), nil
	}

	var req MailboxSetArgs
	if err := json.Unmarshal(args, &req); err != nil {
		return errorResult(ErrInvalidArguments), nil
	}
	if req.AccountID == "" {
		req.AccountID = userID
	}

	oldState, _ := MailboxStateFor(ctx, m.deps.Repo.DB(), userID)
	resp := SetResponse{
		AccountID:    req.AccountID,
		OldState:     oldState,
		Created:      make(map[string]any),
		Updated:      make(map[string]any),
		NotCreated:   make(map[string]SetError),
		NotUpdated:   make(map[string]SetError),
		NotDestroyed: make(map[string]SetError),
	}

	// --- create ---
	for clientID, patch := range req.Create {
		if strings.TrimSpace(patch.Name) == "" {
			resp.NotCreated[clientID] = SetError{Type: "invalidProperties", Description: "name is required"}
			continue
		}
		f, err := m.deps.Repo.CreateFolder(ctx, maildb.CreateFolderRequest{UserID: userID, Name: patch.Name})
		if err != nil {
			resp.NotCreated[clientID] = SetError{Type: "invalidProperties", Description: err.Error()}
			continue
		}
		resp.Created[clientID] = map[string]any{"id": f.ID, "name": f.Name}
	}

	// --- update ---
	for id, patch := range req.Update {
		if strings.TrimSpace(patch.Name) == "" {
			resp.NotUpdated[id] = SetError{Type: "invalidProperties", Description: "name is required for update"}
			continue
		}
		if _, err := m.deps.Repo.RenameFolder(ctx, userID, id, patch.Name); err != nil {
			resp.NotUpdated[id] = SetError{Type: "notFound", Description: err.Error()}
			continue
		}
		resp.Updated[id] = nil
	}

	// --- destroy ---
	for _, id := range req.Destroy {
		if err := m.deps.Repo.DeleteFolder(ctx, userID, id); err != nil {
			resp.NotDestroyed[id] = SetError{Type: "notFound", Description: err.Error()}
			continue
		}
		resp.Destroyed = append(resp.Destroyed, id)
	}

	resp.NewState, _ = MailboxStateFor(ctx, m.deps.Repo.DB(), userID)
	return json.Marshal(resp)
}

// mailboxChangesMethod implements the Mailbox/changes JMAP method (RFC 8621 §2).
type mailboxChangesMethod struct{ deps Deps }

func (m *mailboxChangesMethod) Call(ctx context.Context, userID string, args json.RawMessage) (json.RawMessage, error) {
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

	newState, _ := MailboxStateFor(ctx, m.deps.Repo.DB(), userID)
	resp := ChangesResponse{
		AccountID: req.AccountID,
		OldState:  req.SinceState,
		NewState:  newState,
		Created:   []string{},
		Updated:   []string{},
		Destroyed: []string{},
	}

	// Conservative approach: if the state changed, mark all current mailboxes
	// as updated. We do not have a per-folder change log (modseq) at this time.
	if req.SinceState != newState {
		folders, err := m.deps.Repo.ListFolders(ctx, userID)
		if err == nil {
			for _, f := range folders {
				resp.Updated = append(resp.Updated, f.ID)
			}
		}
	}
	return json.Marshal(resp)
}
