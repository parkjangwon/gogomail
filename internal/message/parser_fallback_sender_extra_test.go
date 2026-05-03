package message

import (
	"testing"
	"time"
)

func TestFallbackMessageIDChangesWithEnvelopeSender(t *testing.T) {
	at := time.Date(2026, 5, 3, 1, 2, 3, 0, time.UTC)
	a := FallbackMessageID("one@example.com", []string{"rcpt@example.com"}, at, "subject")
	b := FallbackMessageID("two@example.com", []string{"rcpt@example.com"}, at, "subject")
	if a == b {
		t.Fatal("FallbackMessageID did not change when envelope sender changed")
	}
}
