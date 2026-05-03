package message

import "testing"

func TestNormalizeMessageIDPreservesExistingBrackets(t *testing.T) {
	if got := normalizeMessageID(" <abc@example.com> "); got != "<abc@example.com>" {
		t.Fatalf("normalizeMessageID = %q, want existing bracketed ID", got)
	}
}
