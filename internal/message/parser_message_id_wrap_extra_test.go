package message

import "testing"

func TestNormalizeMessageIDWrapsBareID(t *testing.T) {
	if got := normalizeMessageID("abc@example.com"); got != "<abc@example.com>" {
		t.Fatalf("normalizeMessageID = %q, want wrapped ID", got)
	}
}
