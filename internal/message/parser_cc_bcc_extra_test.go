package message

import (
	"strings"
	"testing"
)

func TestParseEMLReadsCcAndBccAddressLists(t *testing.T) {
	raw := "From: a@example.com\r\nTo: b@example.com\r\nCc: C <c@example.com>\r\nBcc: d@example.com\r\n\r\nhello"
	parsed, err := ParseEML(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	if len(parsed.Cc) != 1 || parsed.Cc[0].Address != "c@example.com" {
		t.Fatalf("Cc = %+v, want c@example.com", parsed.Cc)
	}
	if len(parsed.Bcc) != 1 || parsed.Bcc[0].Address != "d@example.com" {
		t.Fatalf("Bcc = %+v, want d@example.com", parsed.Bcc)
	}
}
