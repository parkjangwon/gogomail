package outbound

import "testing"

func TestComposeTextPreservesWrappedMessageID(t *testing.T) {
	msg, err := ComposeText(TextMessage{
		From:      Address{Email: "sender@example.com"},
		To:        []Address{{Email: "rcpt@example.net"}},
		MessageID: "<ready@example.com>",
	})
	if err != nil {
		t.Fatalf("ComposeText returned error: %v", err)
	}
	if msg.MessageID != "<ready@example.com>" {
		t.Fatalf("MessageID = %q, want existing ID preserved", msg.MessageID)
	}
}
