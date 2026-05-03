package maildb

import "testing"

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

func TestValidateBulkMessageDeleteRequestRequiresIDs(t *testing.T) {
	t.Parallel()

	err := ValidateBulkMessageDeleteRequest(BulkMessageDeleteRequest{UserID: "user-1"})
	if err == nil {
		t.Fatal("ValidateBulkMessageDeleteRequest accepted missing message IDs")
	}
}
