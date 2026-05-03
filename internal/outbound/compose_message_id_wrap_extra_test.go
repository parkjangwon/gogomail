package outbound

import "testing"

func TestComposeTextWrapsBareMessageID(t *testing.T) {
	msg, err := ComposeText(TextMessage{
		From:      Address{Email: "sender@example.com"},
		To:        []Address{{Email: "rcpt@example.net"}},
		MessageID: "bare@example.com",
	})
	if err != nil {
		t.Fatalf("ComposeText returned error: %v", err)
	}
	if msg.MessageID != "<bare@example.com>" {
		t.Fatalf("MessageID = %q, want wrapped bare ID", msg.MessageID)
	}
}
