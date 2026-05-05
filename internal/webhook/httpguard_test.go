package webhook

import (
	"io"
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

func TestDrainAndCloseIsBounded(t *testing.T) {
	t.Parallel()

	body := &trackingReadCloser{Reader: strings.NewReader("abcdef")}
	if err := DrainAndClose(body, 3); err != nil {
		t.Fatalf("DrainAndClose returned error: %v", err)
	}
	if body.closed != 1 {
		t.Fatalf("closed = %d, want 1", body.closed)
	}
	if body.read != 3 {
		t.Fatalf("read = %d, want 3", body.read)
	}
}

type trackingReadCloser struct {
	*strings.Reader
	closed int
	read   int
}

func (r *trackingReadCloser) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += n
	return n, err
}

func (r *trackingReadCloser) Close() error {
	r.closed++
	return nil
}

var _ io.ReadCloser = (*trackingReadCloser)(nil)
