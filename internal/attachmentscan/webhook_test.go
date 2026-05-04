package attachmentscan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookScannerPostsAttachmentRequest(t *testing.T) {
	t.Parallel()

	var got webhookRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Fatalf("Authorization = %q, want bearer token", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"verdict":" reject ","reason":" blocked "}`))
	}))
	defer server.Close()

	scanner, err := NewWebhookScanner(WebhookOptions{Endpoint: server.URL, Token: " secret-token ", Client: server.Client()})
	if err != nil {
		t.Fatalf("NewWebhookScanner returned error: %v", err)
	}
	result, err := scanner.ScanAttachments(context.Background(), Request{
		RemoteAddr:   "192.0.2.10:25",
		EnvelopeFrom: "sender@example.com",
		Recipients:   []string{"user@example.net"},
		CompanyID:    "company-1",
		DomainID:     "domain-1",
		UserID:       "user-1",
		MessageID:    "<msg@example.com>",
		Subject:      "hello",
		Size:         123,
		Attachments:  []Attachment{{Filename: "report.pdf"}},
	})
	if err != nil {
		t.Fatalf("ScanAttachments returned error: %v", err)
	}
	if result.Verdict != VerdictReject || result.Reason != "blocked" {
		t.Fatalf("result = %+v", result)
	}
	if got.MessageID != "<msg@example.com>" || len(got.Attachments) != 1 || got.Attachments[0].Filename != "report.pdf" {
		t.Fatalf("request = %+v", got)
	}
}

func TestWebhookScannerRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	_, err := decodeWebhookResponse(strings.NewReader(strings.Repeat(" ", int(maxWebhookResponseBytes)+1)))
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("decodeWebhookResponse error = %v, want too large", err)
	}
}

func TestWebhookScannerRejectsTrailingJSON(t *testing.T) {
	t.Parallel()

	_, err := decodeWebhookResponse(strings.NewReader(`{"verdict":"accept"} {"verdict":"reject"}`))
	if err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("decodeWebhookResponse error = %v, want trailing JSON rejection", err)
	}
}

func TestNewWebhookScannerRejectsUnsupportedEndpoint(t *testing.T) {
	t.Parallel()

	if _, err := NewWebhookScanner(WebhookOptions{Endpoint: "ftp://scanner.example/scan"}); err == nil {
		t.Fatal("NewWebhookScanner accepted unsupported endpoint")
	}
}
