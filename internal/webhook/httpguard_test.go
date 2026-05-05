package webhook

import (
	"strings"
	"testing"
)

func TestNormalizeWebhookToken(t *testing.T) {
	t.Parallel()

	got, err := NormalizeWebhookToken(" token-123 ", 16)
	if err != nil {
		t.Fatalf("NormalizeWebhookToken error = %v", err)
	}
	if got != "token-123" {
		t.Fatalf("token = %q, want %q", got, "token-123")
	}

	if _, err := NormalizeWebhookToken("token\nabc", 16); err == nil {
		t.Fatal("wanted error for token containing line break")
	}

	if _, err := NormalizeWebhookToken(strings.Repeat("t", 17), 16); err == nil {
		t.Fatal("wanted error for oversized token")
	}
}

func TestErrorBodyPreview(t *testing.T) {
	t.Parallel()

	preview := ErrorBodyPreview(strings.NewReader("signer failed\r\ntrace-id: 123   456"), 512)
	if preview != "signer failed trace-id: 123 456" {
		t.Fatalf("preview = %q", preview)
	}
}
