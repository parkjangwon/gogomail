package outbound

import (
	"strings"
	"testing"
)

func TestComposeTextRejectsMalformedMessageID(t *testing.T) {
	t.Parallel()

	for _, messageID := range []string{
		"bad id@example.com",
		"<missing-end@example.com",
		"missing-start@example.com>",
		"missing-at",
		"too@many@example.com",
	} {
		messageID := messageID
		t.Run(messageID, func(t *testing.T) {
			t.Parallel()

			_, err := ComposeText(TextMessage{
				From:      Address{Email: "sender@example.com"},
				To:        []Address{{Email: "recipient@example.net"}},
				MessageID: messageID,
				TextBody:  "body",
			})
			if err == nil {
				t.Fatal("ComposeText accepted malformed Message-ID")
			}
		})
	}
}

func TestComposeTextSkipsMalformedThreadMessageIDs(t *testing.T) {
	t.Parallel()

	msg, err := ComposeText(TextMessage{
		From:       Address{Email: "sender@example.com"},
		To:         []Address{{Email: "recipient@example.net"}},
		MessageID:  "reply@example.com",
		InReplyTo:  "bad id@example.com",
		References: []string{"root@example.com", "bad id@example.com", "missing-at"},
		TextBody:   "body",
	})
	if err != nil {
		t.Fatalf("ComposeText returned error: %v", err)
	}
	raw := string(msg.Raw)
	if strings.Contains(raw, "In-Reply-To:") {
		t.Fatalf("malformed In-Reply-To leaked into raw message: %s", raw)
	}
	if !strings.Contains(raw, "References: <root@example.com>\r\n") {
		t.Fatalf("safe References missing: %s", raw)
	}
	if strings.Contains(raw, "bad id") || strings.Contains(raw, "missing-at") {
		t.Fatalf("malformed References leaked into raw message: %s", raw)
	}
}
