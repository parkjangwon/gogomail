package outbound

import (
	"strings"
	"testing"
)

func TestNormalizeCRLFCanonicalizesMixedLineEndings(t *testing.T) {
	got := normalizeCRLF("a\rb\nc\r\nd")
	if got != "a\r\nb\r\nc\r\nd" {
		t.Fatalf("normalizeCRLF = %q, want canonical CRLF", got)
	}
	if strings.Contains(got, "\r\r") {
		t.Fatalf("normalizeCRLF produced doubled carriage returns: %q", got)
	}
}
