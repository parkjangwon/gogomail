package message

import (
	"strings"
	"testing"
)

func TestReadLimitedTextZeroLimitUsesDefault(t *testing.T) {
	body, truncated, err := readLimitedText(strings.NewReader("tiny"), 0)
	if err != nil {
		t.Fatalf("readLimitedText returned error: %v", err)
	}
	if truncated || body != "tiny" {
		t.Fatalf("body=%q truncated=%v, want default limit to keep tiny body", body, truncated)
	}
}
