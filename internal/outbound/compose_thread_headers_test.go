package outbound

import (
	"strings"
	"testing"
	"time"
)

func TestComposeTextWritesThreadHeaders(t *testing.T) {
	t.Parallel()

	msg, err := ComposeText(TextMessage{
		From:       Address{Email: "sender@example.com"},
		To:         []Address{{Email: "recipient@example.net"}},
		Subject:    "Re: hello",
		TextBody:   "body",
		MessageID:  "<reply@example.com>",
		InReplyTo:  "parent@example.com",
		References: []string{"<root@example.com>", "parent@example.com", "<ROOT@example.com>"},
		Date:       time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ComposeText returned error: %v", err)
	}
	raw := string(msg.Raw)
	if !strings.Contains(raw, "In-Reply-To: <parent@example.com>\r\n") {
		t.Fatalf("raw message missing In-Reply-To: %s", raw)
	}
	if !strings.Contains(raw, "References: <root@example.com> <parent@example.com>\r\n") {
		t.Fatalf("raw message missing References: %s", raw)
	}
}

func TestComposeTextSkipsUnsafeThreadHeaders(t *testing.T) {
	t.Parallel()

	msg, err := ComposeText(TextMessage{
		From:       Address{Email: "sender@example.com"},
		To:         []Address{{Email: "recipient@example.net"}},
		Subject:    "Re: hello",
		TextBody:   "body",
		MessageID:  "<reply@example.com>",
		InReplyTo:  "bad\r\nInjected: yes",
		References: []string{"ok@example.com", "bad\nid@example.com"},
		Date:       time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ComposeText returned error: %v", err)
	}
	raw := string(msg.Raw)
	if strings.Contains(raw, "Injected: yes") || strings.Contains(raw, "In-Reply-To:") {
		t.Fatalf("unsafe In-Reply-To leaked into raw message: %s", raw)
	}
	if !strings.Contains(raw, "References: <ok@example.com>\r\n") {
		t.Fatalf("safe References missing: %s", raw)
	}
}
