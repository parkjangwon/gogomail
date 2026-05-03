package message

import (
	"strings"
	"testing"
)

func TestParseEMLAllowsMissingSubject(t *testing.T) {
	raw := "From: a@example.com\r\nTo: b@example.com\r\n\r\nhello"
	parsed, err := ParseEML(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	if parsed.Subject != "" || parsed.TextBody != "hello" {
		t.Fatalf("parsed subject/body = %q/%q, want empty subject and body", parsed.Subject, parsed.TextBody)
	}
}
