package attachmentscan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
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

func TestNewWebhookScannerRejectsEndpointWithLineBreak(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be made when endpoint is invalid")
	}))
	defer server.Close()

	if _, err := NewWebhookScanner(WebhookOptions{Endpoint: "http://scanner.example/scan\npath", Client: server.Client()}); err == nil {
		t.Fatal("NewWebhookScanner accepted endpoint with line break")
	}
}

func TestWebhookScannerReturnsHTTPFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "scanner failed\ntrace-id: abc", http.StatusBadGateway)
	}))
	defer server.Close()

	scanner, err := NewWebhookScanner(WebhookOptions{Endpoint: server.URL, Client: server.Client()})
	if err != nil {
		t.Fatalf("NewWebhookScanner returned error: %v", err)
	}
	_, err = scanner.ScanAttachments(context.Background(), Request{
		EnvelopeFrom: "sender@example.com",
		Recipients:   []string{"user@example.net"},
	})
	if err == nil ||
		!strings.Contains(err.Error(), "HTTP 502") ||
		!strings.Contains(err.Error(), "scanner failed trace-id: abc") ||
		strings.ContainsAny(err.Error(), "\r\n") {
		t.Fatalf("ScanAttachments error = %v, want HTTP 502 with sanitized body", err)
	}
}

func TestNewWebhookScannerRejectsTokenWithLineBreak(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("request should not be made when token is invalid")
	}))
	defer server.Close()

	if _, err := NewWebhookScanner(WebhookOptions{Endpoint: server.URL, Token: "scanner-token\rabc", Client: server.Client()}); err == nil {
		t.Fatal("NewWebhookScanner accepted token with line break")
	}
}

func TestWebhookScannerBoundsRequestPayload(t *testing.T) {
	t.Parallel()

	var got webhookRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"verdict":"accept"}`))
	}))
	defer server.Close()

	scanner, err := NewWebhookScanner(WebhookOptions{Endpoint: server.URL, Client: server.Client()})
	if err != nil {
		t.Fatalf("NewWebhookScanner returned error: %v", err)
	}
	recipients := make([]string, 0, maxWebhookRecipients+2)
	for i := 0; i < maxWebhookRecipients+2; i++ {
		recipients = append(recipients, "user@example.net\r\nBcc: hidden@example.net")
	}
	attachments := make([]Attachment, 0, maxWebhookAttachments+2)
	for i := 0; i < maxWebhookAttachments+2; i++ {
		attachments = append(attachments, Attachment{Filename: strings.Repeat("\u20ac", maxWebhookAttachmentNameBytes)})
	}
	_, err = scanner.ScanAttachments(context.Background(), Request{
		RemoteAddr:     strings.Repeat("r", maxWebhookRemoteAddrBytes) + "\nextra",
		EnvelopeFrom:   "sender@example.com\r\nX-Injected: yes",
		Recipients:     recipients,
		CompanyID:      strings.Repeat("c", maxWebhookIdentityBytes) + "\nextra",
		DomainID:       "domain-1\nextra",
		UserID:         "user-1\nextra",
		SubmissionUser: "submission@example.com\r\nX-Injected: yes",
		MessageID:      strings.Repeat("m", maxWebhookMessageIDBytes) + "\nextra",
		Subject:        strings.Repeat("\u20ac", maxWebhookSubjectBytes),
		Size:           -1,
		Attachments:    attachments,
	})
	if err != nil {
		t.Fatalf("ScanAttachments returned error: %v", err)
	}
	joined := got.RemoteAddr + got.EnvelopeFrom + strings.Join(got.Recipients, "") +
		got.CompanyID + got.DomainID + got.UserID + got.SubmissionUser + got.MessageID + got.Subject
	if strings.ContainsAny(joined, "\r\n") {
		t.Fatalf("request contains line break: %+v", got)
	}
	if len(got.RemoteAddr) > maxWebhookRemoteAddrBytes || len(got.MessageID) > maxWebhookMessageIDBytes {
		t.Fatalf("remote/message lengths = %d/%d", len(got.RemoteAddr), len(got.MessageID))
	}
	if len(got.Recipients) != maxWebhookRecipients {
		t.Fatalf("recipients = %d, want %d", len(got.Recipients), maxWebhookRecipients)
	}
	if len(got.Attachments) != maxWebhookAttachments {
		t.Fatalf("attachments = %d, want %d", len(got.Attachments), maxWebhookAttachments)
	}
	if got.Size != 0 {
		t.Fatalf("size = %d, want 0", got.Size)
	}
	if len(got.Subject) > maxWebhookSubjectBytes || !utf8.ValidString(got.Subject) {
		t.Fatalf("subject length/utf8 = %d/%v", len(got.Subject), utf8.ValidString(got.Subject))
	}
	if len(got.Attachments[0].Filename) > maxWebhookAttachmentNameBytes || !utf8.ValidString(got.Attachments[0].Filename) {
		t.Fatalf("filename length/utf8 = %d/%v", len(got.Attachments[0].Filename), utf8.ValidString(got.Attachments[0].Filename))
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
