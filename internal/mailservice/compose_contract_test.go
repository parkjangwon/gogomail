package mailservice

import (
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
