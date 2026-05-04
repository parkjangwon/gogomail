package message

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseEMLWithOptionsLimitsRetainedMetadata(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"Message-ID: <" + strings.Repeat("m", 40) + "@example.com>",
		"From: " + strings.Repeat("Sender", 8) + " <" + strings.Repeat("local", 8) + "@example.net>",
		"To: " + strings.Repeat("Recipient", 8) + " <" + strings.Repeat("target", 8) + "@example.com>",
		"References: <root@example.com> <" + strings.Repeat("r", 40) + "@example.com>",
		"Subject: hello 한글 subject",
		"",
		"body",
	}, "\r\n")

	parsed, err := ParseEMLWithOptions(strings.NewReader(raw), ParseOptions{MaxMetadataBytes: 20})
	if err != nil {
		t.Fatalf("ParseEMLWithOptions returned error: %v", err)
	}
	if parsed.MessageID != "" {
		t.Fatalf("MessageID = %q, want oversized id dropped", parsed.MessageID)
	}
	if len(parsed.Subject) > 20 {
		t.Fatalf("Subject = %q (%d bytes), want bounded subject", parsed.Subject, len(parsed.Subject))
	}
	if len(parsed.From.Name) > 20 || len(parsed.From.Address) > 20 {
		t.Fatalf("From = %+v, want bounded metadata", parsed.From)
	}
	if len(parsed.To) != 1 || len(parsed.To[0].Name) > 20 || len(parsed.To[0].Address) > 20 {
		t.Fatalf("To = %+v, want bounded metadata", parsed.To)
	}
	if got := strings.Join(parsed.References, ","); got != "<root@example.com>" {
		t.Fatalf("References = %q, want oversized reference dropped", got)
	}
	if !parsed.MetadataTruncated {
		t.Fatal("MetadataTruncated = false, want true")
	}
	if !utf8.ValidString(parsed.Subject) || !utf8.ValidString(parsed.From.Name) || !utf8.ValidString(parsed.To[0].Name) {
		t.Fatalf("metadata contains invalid UTF-8: %+v", parsed)
	}
}

func TestParseEMLWithOptionsPreboundsStructuredHeaderParsing(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"From: Sender <sender@example.net>",
		"To: " + strings.Repeat("Recipient <recipient@example.com>, ", 96),
		"References: " + strings.Repeat("<reference@example.com> ", 96),
		"Subject: oversized structured headers",
		"",
		"body",
	}, "\r\n")

	parsed, err := ParseEMLWithOptions(strings.NewReader(raw), ParseOptions{
		MaxHeaderBytes:   int64(len(raw) + 1024),
		MaxMetadataBytes: 32,
	})
	if err != nil {
		t.Fatalf("ParseEMLWithOptions returned error: %v", err)
	}
	if len(parsed.To) != 0 {
		t.Fatalf("To = %+v, want oversized structured header skipped before address parsing", parsed.To)
	}
	if len(parsed.References) != 0 {
		t.Fatalf("References = %+v, want oversized structured header skipped before msg-id parsing", parsed.References)
	}
	if !parsed.AddressesTruncated || !parsed.ReferencesTruncated || !parsed.MetadataTruncated {
		t.Fatalf("truncation flags = addresses:%v references:%v metadata:%v", parsed.AddressesTruncated, parsed.ReferencesTruncated, parsed.MetadataTruncated)
	}
}

func TestParseEMLWithOptionsPreboundsSubjectBeforeDecoding(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"From: Sender <sender@example.net>",
		"To: Recipient <recipient@example.com>",
		"Subject: " + strings.Repeat("s", 256),
		"",
		"body",
	}, "\r\n")

	parsed, err := ParseEMLWithOptions(strings.NewReader(raw), ParseOptions{MaxMetadataBytes: 32})
	if err != nil {
		t.Fatalf("ParseEMLWithOptions returned error: %v", err)
	}
	if parsed.Subject != "" {
		t.Fatalf("Subject = %q, want oversized subject skipped before decoding", parsed.Subject)
	}
	if !parsed.MetadataTruncated {
		t.Fatal("MetadataTruncated = false, want true")
	}
}

func TestSanitizeHeaderMetadataRemovesControlCharacters(t *testing.T) {
	t.Parallel()

	got, truncated := sanitizeHeaderMetadata(" hello\r\n\t\x00world ", 64, false)
	if got != "hello world" {
		t.Fatalf("sanitizeHeaderMetadata = %q, want %q", got, "hello world")
	}
	if !truncated {
		t.Fatal("truncated = false, want true for cleaned controls")
	}
}
