package message

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestReadLimitedTextTruncatesAtUTF8Boundary(t *testing.T) {
	body, truncated, err := readLimitedText(strings.NewReader("hello 한글"), 8)
	if err != nil {
		t.Fatalf("readLimitedText returned error: %v", err)
	}
	if !truncated {
		t.Fatal("truncated = false, want true")
	}
	if !utf8.ValidString(body) {
		t.Fatalf("body is invalid UTF-8: %q", body)
	}
	if body != "hello " {
		t.Fatalf("body = %q, want %q", body, "hello ")
	}
}
