package outbound

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/message"
)

func TestComposeTextBuildsParseableRFC5322Message(t *testing.T) {
	t.Parallel()

	composed, err := ComposeText(TextMessage{
		From:     Address{Name: "Sender", Email: "sender@example.com"},
		To:       []Address{{Name: "User", Email: "user@example.com"}},
		Subject:  "hello 안녕",
		TextBody: "line1\nline2",
		Date:     time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ComposeText returned error: %v", err)
	}
	if !bytes.Contains(composed.Raw, []byte("\r\n")) {
		t.Fatal("composed message does not use CRLF line endings")
	}
	if !strings.HasPrefix(composed.MessageID, "<") || !strings.HasSuffix(composed.MessageID, ">") {
		t.Fatalf("MessageID = %q, want angle-address form", composed.MessageID)
	}

	parsed, err := message.ParseEML(bytes.NewReader(composed.Raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	if parsed.Subject != "hello 안녕" {
		t.Fatalf("Subject = %q", parsed.Subject)
	}
	if !strings.Contains(parsed.TextBody, "line1") || !strings.Contains(parsed.TextBody, "line2") {
		t.Fatalf("TextBody = %q", parsed.TextBody)
	}
}

func TestComposeTextRejectsMissingRecipients(t *testing.T) {
	t.Parallel()

	_, err := ComposeText(TextMessage{
		From:    Address{Email: "sender@example.com"},
		Subject: "hello",
	})
	if err == nil {
		t.Fatal("ComposeText accepted missing recipients")
	}
}

func TestComposeTextRejectsInvalidRecipientAddress(t *testing.T) {
	t.Parallel()

	_, err := ComposeText(TextMessage{
		From:     Address{Email: "sender@example.com"},
		To:       []Address{{Email: "not an address"}},
		Subject:  "hello",
		TextBody: "body",
	})
	if err == nil {
		t.Fatal("ComposeText accepted invalid recipient address")
	}
}
