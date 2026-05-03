package smtpd

import "testing"

func TestSanitizeReceivedTokenRemovesNewlines(t *testing.T) {
	if got := sanitizeReceivedToken(" mx\r\n forged "); got != "mx forged" {
		t.Fatalf("sanitizeReceivedToken = %q, want newlines removed and trimmed", got)
	}
}
