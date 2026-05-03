package message

import "testing"

func TestNormalizeMessageIDKeepsBlankEmpty(t *testing.T) {
	if got := normalizeMessageID(" \t "); got != "" {
		t.Fatalf("normalizeMessageID = %q, want empty", got)
	}
}
