package message

import (
	"strings"
	"testing"
)

func BenchmarkParseEMLSimpleText(b *testing.B) {
	raw := strings.Join([]string{
		"Message-ID: <bench@example.com>",
		"Date: Sun, 03 May 2026 09:00:00 +0000",
		"From: Sender <sender@example.net>",
		"To: Admin <admin@example.com>",
		"Subject: benchmark",
		"Content-Type: text/plain; charset=utf-8",
		"",
		strings.Repeat("hello body\n", 64),
	}, "\r\n")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ParseEML(strings.NewReader(raw)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseEMLMetadataOnly(b *testing.B) {
	raw := strings.Join([]string{
		"Message-ID: <bench@example.com>",
		"Date: Sun, 03 May 2026 09:00:00 +0000",
		"From: Sender <sender@example.net>",
		"To: Admin <admin@example.com>",
		"Subject: benchmark",
		"Content-Type: text/plain; charset=utf-8",
		"",
		strings.Repeat("hello body\n", 64),
	}, "\r\n")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ParseEMLWithOptions(strings.NewReader(raw), ParseOptions{SkipTextBody: true}); err != nil {
			b.Fatal(err)
		}
	}
}
