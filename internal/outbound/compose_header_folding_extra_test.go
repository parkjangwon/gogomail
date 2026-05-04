package outbound

import (
	"bytes"
	"strings"
	"testing"
)

func TestComposeTextFoldsLongRFC5322Headers(t *testing.T) {
	t.Parallel()

	recipients := make([]Address, 0, 80)
	for i := 0; i < 80; i++ {
		recipients = append(recipients, Address{
			Name:  "Recipient " + strings.Repeat("Name", 4),
			Email: "recipient-" + strings.Repeat("x", 20) + "-" + string(rune('a'+i%26)) + "@example.net",
		})
	}
	composed, err := ComposeText(TextMessage{
		From:     Address{Name: "Sender", Email: "sender@example.com"},
		To:       recipients,
		Subject:  strings.Repeat("Long subject ", 140),
		TextBody: "body",
	})
	if err != nil {
		t.Fatalf("ComposeText returned error: %v", err)
	}
	headerBlock, _, _ := bytes.Cut(composed.Raw, []byte("\r\n\r\n"))
	for _, line := range bytes.Split(headerBlock, []byte("\r\n")) {
		if len(line) > maxHeaderLineBytes {
			t.Fatalf("header line length = %d, want <= %d: %.80q", len(line), maxHeaderLineBytes, line)
		}
	}
	if !bytes.Contains(headerBlock, []byte("\r\n\t")) {
		t.Fatalf("headers were not folded:\n%s", headerBlock)
	}
}
