package jmap

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/maildb"
)

// TestFolderToMailboxInboxGetsInboxRole verifies that a system inbox folder
// gets the "inbox" JMAP role and that system folders are not deletable.
func TestFolderToMailboxInboxGetsInboxRole(t *testing.T) {
	f := maildb.Folder{
		ID:         "f1",
		Name:       "INBOX",
		SystemType: "inbox",
		Total:      5,
		Unread:     2,
	}
	mb := folderToMailbox(f)
	if mb.ID != "f1" {
		t.Errorf("id: want f1, got %s", mb.ID)
	}
	if mb.Role == nil || *mb.Role != "inbox" {
		t.Errorf("role: want inbox, got %v", mb.Role)
	}
	if mb.TotalEmails != 5 {
		t.Errorf("totalEmails: want 5, got %d", mb.TotalEmails)
	}
	if mb.UnreadEmails != 2 {
		t.Errorf("unreadEmails: want 2, got %d", mb.UnreadEmails)
	}
	if mb.MyRights.MayDelete {
		t.Error("system folder should not be deletable (MayDelete must be false)")
	}
}

// TestFolderToMailboxUserFolderHasNoRole verifies that a user-created folder
// has no JMAP role and is deletable.
func TestFolderToMailboxUserFolderHasNoRole(t *testing.T) {
	f := maildb.Folder{ID: "f2", Name: "Work", SystemType: ""}
	mb := folderToMailbox(f)
	if mb.Role != nil {
		t.Errorf("user folder should have nil role, got %v", *mb.Role)
	}
	if !mb.MyRights.MayDelete {
		t.Error("user folder should be deletable (MayDelete must be true)")
	}
}

// TestFolderToMailboxParentID verifies that a non-empty ParentID is surfaced
// as a non-nil pointer on the resulting Mailbox.
func TestFolderToMailboxParentID(t *testing.T) {
	f := maildb.Folder{ID: "f3", Name: "Sub", ParentID: "parent1"}
	mb := folderToMailbox(f)
	if mb.ParentID == nil || *mb.ParentID != "parent1" {
		t.Errorf("parentId: want parent1, got %v", mb.ParentID)
	}
}

// TestMailboxGetArgsDecodeIDs verifies that MailboxGetArgs correctly decodes
// an ids array from the raw JSON arguments object.
func TestMailboxGetArgsDecodeIDs(t *testing.T) {
	raw := json.RawMessage(`{"accountId":"u1","ids":["f1","f2"]}`)
	var args MailboxGetArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	if len(args.IDs) != 2 {
		t.Errorf("want 2 ids, got %d", len(args.IDs))
	}
	if args.IDs[0] != "f1" || args.IDs[1] != "f2" {
		t.Errorf("unexpected ids: %v", args.IDs)
	}
}

// TestChangesResponseMarshal verifies that a ChangesResponse with a non-empty
// updated list serialises to valid JSON containing the updated entry.
func TestChangesResponseMarshal(t *testing.T) {
	resp := ChangesResponse{
		AccountID: "u1",
		OldState:  "1",
		NewState:  "2",
		Created:   []string{},
		Updated:   []string{"f1"},
		Destroyed: []string{},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"updated":["f1"]`) {
		t.Errorf("missing updated field in marshalled response: %s", s)
	}
	if !strings.Contains(s, `"hasMoreChanges":false`) {
		t.Errorf("missing hasMoreChanges field: %s", s)
	}
}

// TestSetErrorMarshal verifies that a SetError serialises to JSON with the
// correct type and description fields.
func TestSetErrorMarshal(t *testing.T) {
	e := SetError{Type: "invalidProperties", Description: "name is required"}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "invalidProperties") {
		t.Errorf("missing type in marshalled SetError: %s", s)
	}
	if !strings.Contains(s, "name is required") {
		t.Errorf("missing description in marshalled SetError: %s", s)
	}
}
