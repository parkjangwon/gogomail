package outbound

import (
	"strings"
	"testing"
)

func TestComposeTextGeneratesMessageIDFromSenderDomain(t *testing.T) {
	msg, err := ComposeText(TextMessage{
		From: Address{Email: "sender@example.com"},
		To:   []Address{{Email: "rcpt@example.net"}},
	})
	if err != nil {
		t.Fatalf("ComposeText returned error: %v", err)
	}
	if !strings.HasSuffix(msg.MessageID, "@example.com>") {
		t.Fatalf("MessageID = %q, want sender domain", msg.MessageID)
	}
}
