package message

import (
	"strings"
	"testing"
)

func TestReadLimitedTextDoesNotTruncateAtExactLimit(t *testing.T) {
	body, truncated, err := readLimitedText(strings.NewReader("12345"), 5)
	if err != nil {
		t.Fatalf("readLimitedText returned error: %v", err)
	}
	if truncated || body != "12345" {
		t.Fatalf("body=%q truncated=%v, want exact body without truncation", body, truncated)
	}
}
