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

func TestValidateSaveDraftRequestRejectsUnsafeResourceIDs(t *testing.T) {
	t.Parallel()

	tests := []SaveDraftRequest{
		{UserID: "user-1", DraftID: "draft-1\r\nbad"},
		{UserID: "user-1", Intent: ComposeIntentReply, SourceMessageID: strings.Repeat("x", maxServiceResourceIDBytes+1)},
	}
	for _, req := range tests {
		if err := ValidateSaveDraftRequest(req); err == nil {
			t.Fatalf("ValidateSaveDraftRequest accepted unsafe ids %+v", req)
		}
	}
}

func TestValidateSaveDraftRequestRejectsTooManyAttachments(t *testing.T) {
	t.Parallel()

	ids := make([]string, MaxComposeAttachments+1)
	for i := range ids {
		ids[i] = "att"
	}
	err := ValidateSaveDraftRequest(SaveDraftRequest{
		UserID:        "user-1",
		AttachmentIDs: ids,
	})
	if err == nil {
		t.Fatal("ValidateSaveDraftRequest accepted too many attachment ids")
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

func TestValidateSaveDraftRequestRejectsInvalidRecipientEmail(t *testing.T) {
	t.Parallel()

	err := ValidateSaveDraftRequest(SaveDraftRequest{
		UserID: "user-1",
		Cc:     []outbound.Address{{Email: "bad address"}},
	})
	if err == nil {
		t.Fatal("ValidateSaveDraftRequest accepted invalid recipient email")
	}
}

func TestValidateSaveDraftRequestRejectsScalarHeaderInjection(t *testing.T) {
	t.Parallel()

	tests := []SaveDraftRequest{
		{
			UserID:  "user-1",
			From:    "sender@example.net\r\nBcc: victim@example.net",
			Subject: "hello",
		},
		{
			UserID:  "user-1",
			Subject: "hello\nBcc: victim@example.net",
		},
	}
	for _, req := range tests {
		if err := ValidateSaveDraftRequest(req); err == nil {
			t.Fatalf("ValidateSaveDraftRequest accepted %+v", req)
		}
	}
}

func TestValidateSaveDraftRequestRejectsRecipientHeaderInjection(t *testing.T) {
	t.Parallel()

	err := ValidateSaveDraftRequest(SaveDraftRequest{
		UserID: "user-1",
		Cc: []outbound.Address{{
			Name:  "Recipient\nBcc: victim@example.net",
			Email: "user@example.net",
		}},
	})
	if err == nil {
		t.Fatal("ValidateSaveDraftRequest accepted newline-bearing recipient name")
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

func TestValidateSaveDraftRequestRejectsTooManyRecipients(t *testing.T) {
	t.Parallel()

	recipients := make([]outbound.Address, MaxComposeRecipients+1)
	for i := range recipients {
		recipients[i] = outbound.Address{Email: "user@example.net"}
	}
	err := ValidateSaveDraftRequest(SaveDraftRequest{
		UserID: "user-1",
		Bcc:    recipients,
	})
	if err == nil {
		t.Fatal("ValidateSaveDraftRequest accepted too many recipients")
	}
}

func TestValidateDeleteDraftRequestRequiresDraftID(t *testing.T) {
	t.Parallel()

	if err := ValidateDeleteDraftRequest("user-1", " "); err == nil {
		t.Fatal("ValidateDeleteDraftRequest accepted blank draft id")
	}
}
