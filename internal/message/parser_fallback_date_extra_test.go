package message

import (
	"testing"
	"time"
)

func TestFallbackMessageIDChangesWithDate(t *testing.T) {
	a := FallbackMessageID("sender@example.com", []string{"rcpt@example.com"}, time.Unix(1, 0), "subject")
	b := FallbackMessageID("sender@example.com", []string{"rcpt@example.com"}, time.Unix(2, 0), "subject")
	if a == b {
		t.Fatal("FallbackMessageID did not change when date changed")
	}
}
