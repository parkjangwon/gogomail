package message

import (
	"strings"
	"testing"
)

func TestParseEMLReadsTextPartWithUnknownCharset(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"From: sender@example.net",
		"To: user@example.com",
		"Subject: unknown charset",
		"Content-Type: multipart/alternative; boundary=frontier",
		"",
		"--frontier",
		"Content-Type: text/plain; charset=x-unknown-gogomail-test",
		"",
		"plain body",
		"--frontier",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>html body</p>",
		"--frontier--",
	}, "\r\n")

	parsed, err := ParseEMLWithOptions(strings.NewReader(raw), ParseOptions{MaxTextBodyBytes: 64})
	if err != nil {
		t.Fatalf("ParseEMLWithOptions returned error: %v", err)
	}
	if parsed.TextBody != "plain body" {
		t.Fatalf("TextBody = %q, want raw body despite unknown charset", parsed.TextBody)
	}
}
