package message

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseEMLSanitizesAttachmentFilenameMetadata(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"From: sender@example.net",
		"To: admin@example.com",
		"Subject: filename hygiene",
		"Content-Type: multipart/mixed; boundary=frontier",
		"",
		"--frontier",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"body",
		"--frontier",
		"Content-Type: text/plain",
		"Content-Disposition: attachment; filename=\"../../reports/quarterly\r\n plan.txt\"",
		"",
		"attachment",
		"--frontier--",
	}, "\r\n")

	parsed, err := ParseEML(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	if len(parsed.Attachments) != 1 {
		t.Fatalf("Attachments = %+v", parsed.Attachments)
	}
	if parsed.Attachments[0].Filename != "quarterly plan.txt" {
		t.Fatalf("Filename = %q, want sanitized basename", parsed.Attachments[0].Filename)
	}
}

func TestParseEMLTruncatesLongAttachmentFilenameAtUTF8Boundary(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"From: sender@example.net",
		"To: admin@example.com",
		"Subject: long filename",
		"Content-Type: multipart/mixed; boundary=frontier",
		"",
		"--frontier",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"body",
		"--frontier",
		"Content-Type: text/plain",
		"Content-Disposition: attachment; filename=\"" + strings.Repeat("a", 254) + "한글.txt\"",
		"",
		"attachment",
		"--frontier--",
	}, "\r\n")

	parsed, err := ParseEML(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	if len(parsed.Attachments) != 1 {
		t.Fatalf("Attachments = %+v", parsed.Attachments)
	}
	filename := parsed.Attachments[0].Filename
	if len(filename) > maxAttachmentFilenameBytes {
		t.Fatalf("filename length = %d, want <= %d", len(filename), maxAttachmentFilenameBytes)
	}
	if !utf8.ValidString(filename) {
		t.Fatalf("filename is invalid UTF-8: %q", filename)
	}
}
