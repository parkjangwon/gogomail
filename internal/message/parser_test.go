package message

import (
	"strings"
	"testing"
	"time"
)

func TestParseEMLExtractsRFC5322MetadataAndTextBody(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"Message-ID: <abc123@example.com>",
		"Date: Sun, 03 May 2026 09:00:00 +0000",
		"From: Sender <sender@example.net>",
		"To: Admin <admin@example.com>",
		"Cc: Copy <copy@example.com>",
		"Subject: =?UTF-8?B?7JWI64WV?=",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"hello body",
	}, "\r\n")

	parsed, err := ParseEML(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}

	if parsed.MessageID != "<abc123@example.com>" {
		t.Fatalf("MessageID = %q", parsed.MessageID)
	}
	if parsed.Subject != "안녕" {
		t.Fatalf("Subject = %q, want 안녕", parsed.Subject)
	}
	if parsed.From.Address != "sender@example.net" || parsed.From.Name != "Sender" {
		t.Fatalf("From = %+v", parsed.From)
	}
	if len(parsed.To) != 1 || parsed.To[0].Address != "admin@example.com" {
		t.Fatalf("To = %+v", parsed.To)
	}
	if len(parsed.Cc) != 1 || parsed.Cc[0].Address != "copy@example.com" {
		t.Fatalf("Cc = %+v", parsed.Cc)
	}
	if parsed.Date.IsZero() || parsed.Date.UTC() != time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC) {
		t.Fatalf("Date = %s", parsed.Date)
	}
	if parsed.TextBody != "hello body" {
		t.Fatalf("TextBody = %q", parsed.TextBody)
	}
}

func TestParseEMLDetectsAttachments(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"From: sender@example.net",
		"To: admin@example.com",
		"Subject: attachment",
		"Content-Type: multipart/mixed; boundary=frontier",
		"",
		"--frontier",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"see attachment",
		"--frontier",
		"Content-Type: text/plain",
		"Content-Disposition: attachment; filename=\"hello.txt\"",
		"",
		"hello file",
		"--frontier--",
	}, "\r\n")

	parsed, err := ParseEML(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}

	if parsed.TextBody != "see attachment" {
		t.Fatalf("TextBody = %q", parsed.TextBody)
	}
	if !parsed.HasAttachment {
		t.Fatal("HasAttachment = false, want true")
	}
	if len(parsed.Attachments) != 1 || parsed.Attachments[0].Filename != "hello.txt" {
		t.Fatalf("Attachments = %+v", parsed.Attachments)
	}
}

func TestParseEMLReadsSinglepartTextWithoutDate(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"Message-ID: <no-date@example.com>",
		"From: sender@example.net",
		"To: admin@example.com",
		"Subject: no date",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"hello without date",
	}, "\r\n")

	parsed, err := ParseEML(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	if parsed.TextBody != "hello without date" {
		t.Fatalf("TextBody = %q", parsed.TextBody)
	}
}

func TestParseEMLWithOptionsLimitsTextBody(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"From: sender@example.net",
		"To: admin@example.com",
		"Subject: large body",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"0123456789abcdef",
	}, "\r\n")

	parsed, err := ParseEMLWithOptions(strings.NewReader(raw), ParseOptions{MaxTextBodyBytes: 8})
	if err != nil {
		t.Fatalf("ParseEMLWithOptions returned error: %v", err)
	}
	if parsed.TextBody != "01234567" {
		t.Fatalf("TextBody = %q, want limited prefix", parsed.TextBody)
	}
	if !parsed.TextBodyTruncated {
		t.Fatal("TextBodyTruncated = false, want true")
	}
}

func TestParseEMLWithOptionsSkipsTextBody(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"From: sender@example.net",
		"To: admin@example.com",
		"Subject: metadata only",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"body should not be extracted",
	}, "\r\n")

	parsed, err := ParseEMLWithOptions(strings.NewReader(raw), ParseOptions{SkipTextBody: true})
	if err != nil {
		t.Fatalf("ParseEMLWithOptions returned error: %v", err)
	}
	if parsed.Subject != "metadata only" {
		t.Fatalf("Subject = %q", parsed.Subject)
	}
	if parsed.TextBody != "" {
		t.Fatalf("TextBody = %q, want empty", parsed.TextBody)
	}
}

func TestParseEMLWithOptionsLimitsAttachmentMetadata(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"From: sender@example.net",
		"To: admin@example.com",
		"Subject: many attachments",
		"Content-Type: multipart/mixed; boundary=frontier",
		"",
		"--frontier",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"body",
		"--frontier",
		"Content-Type: text/plain",
		"Content-Disposition: attachment; filename=\"one.txt\"",
		"",
		"one",
		"--frontier",
		"Content-Type: text/plain",
		"Content-Disposition: attachment; filename=\"two.txt\"",
		"",
		"two",
		"--frontier--",
	}, "\r\n")

	parsed, err := ParseEMLWithOptions(strings.NewReader(raw), ParseOptions{MaxAttachments: 1})
	if err != nil {
		t.Fatalf("ParseEMLWithOptions returned error: %v", err)
	}
	if !parsed.HasAttachment {
		t.Fatal("HasAttachment = false, want true")
	}
	if len(parsed.Attachments) != 1 || parsed.Attachments[0].Filename != "one.txt" {
		t.Fatalf("Attachments = %+v", parsed.Attachments)
	}
	if !parsed.AttachmentsTruncated {
		t.Fatal("AttachmentsTruncated = false, want true")
	}
}
