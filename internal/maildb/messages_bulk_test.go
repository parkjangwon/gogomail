package maildb

import (
	"strings"
	"testing"
)

func TestValidateBulkMessageFlagRequestRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()

	err := ValidateBulkMessageFlagRequest(BulkMessageFlagRequest{
		UserID:     "user-1",
		MessageIDs: []string{"msg-1", "msg-1"},
		Flag:       "read",
		Value:      true,
	})
	if err == nil {
		t.Fatal("ValidateBulkMessageFlagRequest accepted duplicate message IDs")
	}
}

func TestValidateBulkMessageFlagRequestRejectsUnsafeIDs(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"msg-1\r\nmsg-2"},
		{strings.Repeat("x", maxMailboxResourceIDBytes+1)},
	}
	for _, ids := range tests {
		err := ValidateBulkMessageFlagRequest(BulkMessageFlagRequest{
			UserID:     "user-1",
			MessageIDs: ids,
			Flag:       "read",
			Value:      true,
		})
		if err == nil {
			t.Fatalf("ValidateBulkMessageFlagRequest accepted unsafe ids %+v", ids)
		}
	}
}

func TestValidateBulkThreadFlagRequestRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()

	err := ValidateBulkThreadFlagRequest(BulkThreadFlagRequest{
		UserID:    "user-1",
		ThreadIDs: []string{"thread-1", "thread-1"},
		Flag:      "read",
		Value:     true,
	})
	if err == nil {
		t.Fatal("ValidateBulkThreadFlagRequest accepted duplicate thread IDs")
	}
}

func TestValidateBulkThreadFlagRequestRejectsUnsafeIDs(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"thread-1\r\nthread-2"},
		{strings.Repeat("x", maxMailboxResourceIDBytes+1)},
	}
	for _, ids := range tests {
		err := ValidateBulkThreadFlagRequest(BulkThreadFlagRequest{
			UserID:    "user-1",
			ThreadIDs: ids,
			Flag:      "read",
			Value:     true,
		})
		if err == nil {
			t.Fatalf("ValidateBulkThreadFlagRequest accepted unsafe ids %+v", ids)
		}
	}
}

func TestBulkSetThreadFlagSQLUpdatesActiveThreadMessages(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"COALESCE(thread_id, id)::text IN",
		"jsonb_array_elements_text($2::jsonb)",
		"RETURNING id::text",
	} {
		if !strings.Contains(bulkSetThreadFlagSQL, want) {
			t.Fatalf("bulk thread flag SQL does not include %q:\n%s", want, bulkSetThreadFlagSQL)
		}
	}
}

func TestValidateBulkMessageMoveRequestRejectsTooManyIDs(t *testing.T) {
	t.Parallel()

	ids := make([]string, 501)
	for i := range ids {
		ids[i] = "msg"
	}
	err := ValidateBulkMessageMoveRequest(BulkMessageMoveRequest{
		UserID:     "user-1",
		FolderID:   "folder-1",
		MessageIDs: ids,
	})
	if err == nil {
		t.Fatal("ValidateBulkMessageMoveRequest accepted too many message IDs")
	}
}

func TestValidateBulkMessageMoveRequestRejectsUnsafeFolderID(t *testing.T) {
	t.Parallel()

	err := ValidateBulkMessageMoveRequest(BulkMessageMoveRequest{
		UserID:     "user-1",
		FolderID:   strings.Repeat("x", maxMailboxResourceIDBytes+1),
		MessageIDs: []string{"msg-1"},
	})
	if err == nil {
		t.Fatal("ValidateBulkMessageMoveRequest accepted oversized folder ID")
	}
}

func TestValidateBulkMessageDeleteRequestRequiresIDs(t *testing.T) {
	t.Parallel()

	err := ValidateBulkMessageDeleteRequest(BulkMessageDeleteRequest{UserID: "user-1"})
	if err == nil {
		t.Fatal("ValidateBulkMessageDeleteRequest accepted missing message IDs")
	}
}
