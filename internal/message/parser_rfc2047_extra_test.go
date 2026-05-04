package message

import (
	"strings"
	"testing"
)

func TestParseEMLDecodesRFC2047SubjectAndDisplayNames(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"From: =?UTF-8?B?7ISc64qU?= <sender@example.net>",
		"To: =?UTF-8?B?7IiY7Iug?= <user@example.com>",
		"Cc: =?UTF-8?Q?Ops_=ED=8C=80?= <ops@example.com>",
		"Subject: =?UTF-8?B?7ZWc6riAIOygnOuqqQ==?=",
		"Date: Mon, 04 May 2026 09:00:00 +0000",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"body",
	}, "\r\n")

	parsed, err := ParseEML(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	if parsed.Subject != "한글 제목" {
		t.Fatalf("Subject = %q, want decoded RFC 2047 subject", parsed.Subject)
	}
	if parsed.From.Name != "서는" || parsed.From.Address != "sender@example.net" {
		t.Fatalf("From = %+v, want decoded display name and lower-case address", parsed.From)
	}
	if len(parsed.To) != 1 || parsed.To[0].Name != "수신" || parsed.To[0].Address != "user@example.com" {
		t.Fatalf("To = %+v, want decoded recipient", parsed.To)
	}
	if len(parsed.Cc) != 1 || parsed.Cc[0].Name != "Ops 팀" || parsed.Cc[0].Address != "ops@example.com" {
		t.Fatalf("Cc = %+v, want decoded cc recipient", parsed.Cc)
	}
}
