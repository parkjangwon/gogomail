package mailservice

import (
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/outbound"
)

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

func TestValidateSaveDraftRequestRejectsBlankRecipientEmail(t *testing.T) {
	t.Parallel()

	err := ValidateSaveDraftRequest(SaveDraftRequest{
		UserID: "user-1",
		Cc:     []outbound.Address{{Name: "Missing"}},
	})
	if err == nil {
		t.Fatal("ValidateSaveDraftRequest accepted blank recipient email")
	}
	if got := err.Error(); got != "cc[0].email is required" {
		t.Fatalf("error = %q", got)
	}
}

func TestValidateSaveDraftRequestRejectsOversizedTextBody(t *testing.T) {
	t.Parallel()

	err := ValidateSaveDraftRequest(SaveDraftRequest{
		UserID:   "user-1",
		TextBody: strings.Repeat("x", MaxComposeTextBodyBytes+1),
	})
	if err == nil {
		t.Fatal("ValidateSaveDraftRequest accepted oversized text body")
	}
}

func TestValidateDeleteDraftRequestRequiresDraftID(t *testing.T) {
	t.Parallel()

	if err := ValidateDeleteDraftRequest("user-1", " "); err == nil {
		t.Fatal("ValidateDeleteDraftRequest accepted blank draft id")
	}
}
