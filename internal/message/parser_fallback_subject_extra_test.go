package message

import (
	"testing"
	"time"
)

func TestFallbackMessageIDChangesWithSubject(t *testing.T) {
	at := time.Date(2026, 5, 3, 1, 2, 3, 0, time.UTC)
	a := FallbackMessageID("sender@example.com", []string{"rcpt@example.com"}, at, "one")
	b := FallbackMessageID("sender@example.com", []string{"rcpt@example.com"}, at, "two")
	if a == b {
		t.Fatal("FallbackMessageID did not change when subject changed")
	}
}
