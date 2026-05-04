package mailservice

import (
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/outbound"
)

func TestNormalizeComposeIntentDefaultsToNew(t *testing.T) {
	t.Parallel()

	intent, err := NormalizeComposeIntent("")
	if err != nil {
		t.Fatalf("NormalizeComposeIntent returned error: %v", err)
	}
	if intent != ComposeIntentNew {
		t.Fatalf("intent = %q", intent)
	}
}

func TestNormalizeComposeIntentRejectsUnknownValue(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeComposeIntent("bounce"); err == nil {
		t.Fatal("NormalizeComposeIntent accepted unknown value")
	}
}

func TestValidateSendTextRequestRequiresRecipient(t *testing.T) {
	t.Parallel()

	err := ValidateSendTextRequest(SendTextRequest{UserID: "user-1"})
	if err == nil {
		t.Fatal("ValidateSendTextRequest accepted missing recipients")
	}
}

func TestValidateSendTextRequestRejectsBlankRecipientEmail(t *testing.T) {
	t.Parallel()

	err := ValidateSendTextRequest(SendTextRequest{
		UserID: "user-1",
		To:     []outbound.Address{{Name: "Missing"}},
	})
	if err == nil {
		t.Fatal("ValidateSendTextRequest accepted blank recipient email")
	}
	if got := err.Error(); got != "to[0].email is required" {
		t.Fatalf("error = %q", got)
	}
}

func TestValidateSendTextRequestRejectsInvalidRecipientEmail(t *testing.T) {
	t.Parallel()

	err := ValidateSendTextRequest(SendTextRequest{
		UserID: "user-1",
		To:     []outbound.Address{{Email: "not an address"}},
	})
	if err == nil {
		t.Fatal("ValidateSendTextRequest accepted invalid recipient email")
	}
}

func TestValidateSendTextRequestRejectsScalarHeaderInjection(t *testing.T) {
	t.Parallel()

	tests := []SendTextRequest{
		{
			UserID:  "user-1",
			From:    "sender@example.net\r\nBcc: victim@example.net",
			To:      []outbound.Address{{Email: "user@example.net"}},
			Subject: "hello",
		},
		{
			UserID:  "user-1",
			To:      []outbound.Address{{Email: "user@example.net"}},
			Subject: "hello\nBcc: victim@example.net",
		},
	}
	for _, req := range tests {
		if err := ValidateSendTextRequest(req); err == nil {
			t.Fatalf("ValidateSendTextRequest accepted %+v", req)
		}
	}
}

func TestValidateSendTextRequestRejectsRecipientHeaderInjection(t *testing.T) {
	t.Parallel()

	err := ValidateSendTextRequest(SendTextRequest{
		UserID: "user-1",
		To: []outbound.Address{{
			Name:  "Recipient\r\nBcc: victim@example.net",
			Email: "user@example.net",
		}},
	})
	if err == nil {
		t.Fatal("ValidateSendTextRequest accepted newline-bearing recipient name")
	}
}

func TestValidateSendTextRequestRejectsOversizedTextBody(t *testing.T) {
	t.Parallel()

	err := ValidateSendTextRequest(SendTextRequest{
		UserID:   "user-1",
		To:       []outbound.Address{{Email: "user@example.net"}},
		TextBody: strings.Repeat("x", MaxComposeTextBodyBytes+1),
	})
	if err == nil {
		t.Fatal("ValidateSendTextRequest accepted oversized text body")
	}
}

func TestValidateSendTextRequestRejectsTooManyRecipients(t *testing.T) {
	t.Parallel()

	recipients := make([]outbound.Address, MaxComposeRecipients+1)
	for i := range recipients {
		recipients[i] = outbound.Address{Email: "user@example.net"}
	}
	err := ValidateSendTextRequest(SendTextRequest{
		UserID: "user-1",
		To:     recipients,
	})
	if err == nil {
		t.Fatal("ValidateSendTextRequest accepted too many recipients")
	}
}

func TestValidateSendTextRequestRequiresReplySourceMessage(t *testing.T) {
	t.Parallel()

	err := ValidateSendTextRequest(SendTextRequest{
		UserID: "user-1",
		Intent: ComposeIntentReply,
		To:     []outbound.Address{{Email: "sender@example.net"}},
	})
	if err == nil {
		t.Fatal("ValidateSendTextRequest accepted reply without source message")
	}
}

func TestValidateSendTextRequestRejectsUnsafeSourceMessageID(t *testing.T) {
	t.Parallel()

	err := ValidateSendTextRequest(SendTextRequest{
		UserID:          "user-1",
		Intent:          ComposeIntentReply,
		SourceMessageID: "msg-1\nbad",
		To:              []outbound.Address{{Email: "user@example.net"}},
	})
	if err == nil {
		t.Fatal("ValidateSendTextRequest accepted unsafe source message ID")
	}
}

func TestValidateSendTextRequestAcceptsCcOnlyDraftLikeSend(t *testing.T) {
	t.Parallel()

	err := ValidateSendTextRequest(SendTextRequest{
		UserID: "user-1",
		Cc:     []outbound.Address{{Email: "copy@example.net"}},
	})
	if err != nil {
		t.Fatalf("ValidateSendTextRequest returned error: %v", err)
	}
}

func TestValidateSendTextRequestRejectsBlankAttachmentID(t *testing.T) {
	t.Parallel()

	err := ValidateSendTextRequest(SendTextRequest{
		UserID:        "user-1",
		To:            []outbound.Address{{Email: "user@example.net"}},
		AttachmentIDs: []string{" "},
	})
	if err == nil {
		t.Fatal("ValidateSendTextRequest accepted blank attachment id")
	}
}

func TestValidateSendTextRequestRejectsTooManyAttachments(t *testing.T) {
	t.Parallel()

	ids := make([]string, MaxComposeAttachments+1)
	for i := range ids {
		ids[i] = "att"
	}
	err := ValidateSendTextRequest(SendTextRequest{
		UserID:        "user-1",
		To:            []outbound.Address{{Email: "user@example.net"}},
		AttachmentIDs: ids,
	})
	if err == nil {
		t.Fatal("ValidateSendTextRequest accepted too many attachment ids")
	}
}
