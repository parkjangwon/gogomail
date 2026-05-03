package message

import (
	"strings"
	"testing"
)

func TestParseEMLTrimsTrailingCRLFOnly(t *testing.T) {
	raw := "From: a@example.com\r\nTo: b@example.com\r\n\r\nhello\r\n\r\n"
	parsed, err := ParseEML(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	if parsed.TextBody != "hello" {
		t.Fatalf("TextBody = %q, want trailing CRLF trimmed", parsed.TextBody)
	}
}
