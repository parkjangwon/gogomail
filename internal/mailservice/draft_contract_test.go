package mailservice

import "testing"

func TestValidateSaveDraftRequestAllowsEmptyRecipients(t *testing.T) {
	t.Parallel()

	err := ValidateSaveDraftRequest(SaveDraftRequest{
		UserID:   "user-1",
		Subject:  "unfinished",
		TextBody: "draft body",
	})
	if err != nil {
		t.Fatalf("ValidateSaveDraftRequest returned error: %v", err)
	}
}

func TestValidateSaveDraftRequestRejectsBlankAttachmentID(t *testing.T) {
	t.Parallel()

	err := ValidateSaveDraftRequest(SaveDraftRequest{
		UserID:        "user-1",
		AttachmentIDs: []string{"att-1", " "},
	})
	if err == nil {
		t.Fatal("ValidateSaveDraftRequest accepted blank attachment id")
	}
}

func TestValidateDeleteDraftRequestRequiresDraftID(t *testing.T) {
	t.Parallel()

	if err := ValidateDeleteDraftRequest("user-1", " "); err == nil {
		t.Fatal("ValidateDeleteDraftRequest accepted blank draft id")
	}
}
