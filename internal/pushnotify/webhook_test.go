package pushnotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWebhookSinkPostsNotificationTargets(t *testing.T) {
	t.Parallel()

	var got webhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer push-token" {
			t.Fatalf("Authorization = %q, want bearer token", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sink, err := NewWebhookSink(WebhookOptions{Endpoint: server.URL, Token: " push-token ", Client: server.Client(), })
	if err != nil {
		t.Fatalf("NewWebhookSink returned error: %v", err)
	}
	err = sink.EnqueuePush(context.Background(), Notification{
		MessageID:  "msg-1",
		UserID:     "user-1",
		Recipient:  "user@example.net",
		Subject:    "hello",
		ReceivedAt: "2026-05-04T00:00:00Z",
		Targets: []Target{{
			AttemptID:   "attempt-1",
			DeviceID:    "device-1",
			Platform:    "fcm",
			Token:       "raw-token",
			TokenSuffix: "aw-token",
			Label:       "phone",
		}},
	})
	if err != nil {
		t.Fatalf("EnqueuePush returned error: %v", err)
	}
	if got.MessageID != "msg-1" || got.UserID != "user-1" || len(got.Targets) != 1 {
		t.Fatalf("payload = %+v", got)
	}
	if got.Targets[0].AttemptID != "attempt-1" || got.Targets[0].Token != "raw-token" {
		t.Fatalf("target = %+v", got.Targets[0])
	}
}

func TestWebhookSinkBoundsPayloadFields(t *testing.T) {
	t.Parallel()

	var got webhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sink, err := NewWebhookSink(WebhookOptions{Endpoint: server.URL, Client: server.Client(), })
	if err != nil {
		t.Fatalf("NewWebhookSink returned error: %v", err)
	}
	err = sink.EnqueuePush(context.Background(), Notification{
		MessageID:  strings.Repeat("m", maxWebhookIdentityBytes) + "\nextra",
		UserID:     " user-1\ninjected ",
		Recipient:  " user@example.net\r\nBcc: other@example.net ",
		Subject:    strings.Repeat("\u20ac", maxWebhookSubjectBytes),
		ReceivedAt: "2026-05-04T00:00:00Z\nlater",
		Targets: []Target{
			{
				AttemptID: strings.Repeat("a", maxWebhookIdentityBytes) + "\nextra",
				DeviceID:  " device-1 ",
				Platform:  " FCM ",
				Token:     "raw-token",
				Label:     strings.Repeat("\u20ac", maxWebhookLabelBytes),
			},
			{
				DeviceID: "device-2",
				Platform: "fcm",
				Token:    "bad\ntoken",
			},
			{
				DeviceID: "device-3",
				Platform: "unknown",
				Token:    "raw-token",
			},
		},
	})
	if err != nil {
		t.Fatalf("EnqueuePush returned error: %v", err)
	}
	if strings.ContainsAny(got.MessageID+got.UserID+got.Recipient+got.ReceivedAt, "\r\n") {
		t.Fatalf("payload contains line break: %+v", got)
	}
	if len(got.MessageID) > maxWebhookIdentityBytes {
		t.Fatalf("message_id length = %d, want <= %d", len(got.MessageID), maxWebhookIdentityBytes)
	}
	if len(got.Subject) > maxWebhookSubjectBytes || !utf8.ValidString(got.Subject) {
		t.Fatalf("subject length/utf8 = %d/%v", len(got.Subject), utf8.ValidString(got.Subject))
	}
	if len(got.Targets) != 1 {
		t.Fatalf("targets = %+v, want only valid target", got.Targets)
	}
	if got.Targets[0].DeviceID != "device-1" || got.Targets[0].Platform != "fcm" {
		t.Fatalf("target = %+v", got.Targets[0])
	}
	if len(got.Targets[0].AttemptID) > maxWebhookIdentityBytes || strings.ContainsAny(got.Targets[0].AttemptID, "\r\n") {
		t.Fatalf("attempt_id = %q", got.Targets[0].AttemptID)
	}
	if len(got.Targets[0].Label) > maxWebhookLabelBytes || !utf8.ValidString(got.Targets[0].Label) {
		t.Fatalf("label length/utf8 = %d/%v", len(got.Targets[0].Label), utf8.ValidString(got.Targets[0].Label))
	}
}

func TestWebhookSinkReturnsHTTPFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "push failed\ntrace-id: abc", http.StatusBadGateway)
	}))
	defer server.Close()

	sink, err := NewWebhookSink(WebhookOptions{Endpoint: server.URL, Client: server.Client(), })
	if err != nil {
		t.Fatalf("NewWebhookSink returned error: %v", err)
	}
	err = sink.EnqueuePush(context.Background(), Notification{MessageID: "msg-1", UserID: "user-1"})
	if err == nil ||
		!strings.Contains(err.Error(), "HTTP 502") ||
		!strings.Contains(err.Error(), "push failed trace-id: abc") ||
		strings.ContainsAny(err.Error(), "\r\n") {
		t.Fatalf("EnqueuePush error = %v, want HTTP 502", err)
	}
}

func TestNewWebhookSinkRejectsTokenWithLineBreak(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("request should not be made when token is invalid")
	}))
	defer server.Close()

	if _, err := NewWebhookSink(WebhookOptions{Endpoint: server.URL, Token: "push-token\nabc", Client: server.Client(), }); err == nil {
		t.Fatal("NewWebhookSink accepted token with line break")
	}
}

func TestNewWebhookSinkRejectsEndpointWithLineBreak(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be made when endpoint is invalid")
	}))
	defer server.Close()

	if _, err := NewWebhookSink(WebhookOptions{Endpoint: "http://push.example/send\r\npath", Token: "token", Client: server.Client()}); err == nil {
		t.Fatal("NewWebhookSink accepted endpoint with line break")
	}
}

func TestNewWebhookSinkRejectsUnsupportedEndpoint(t *testing.T) {
	t.Parallel()

	if _, err := NewWebhookSink(WebhookOptions{Endpoint: "ftp://push.example/send"}); err == nil {
		t.Fatal("NewWebhookSink accepted unsupported endpoint")
	}
}

// TestWebhookSinkDrainsBodyOnErrorStatus verifies that on a non-2xx response the
// response body is fully read before closing so that the underlying TCP connection
// can be reused by the HTTP client's connection pool.
func TestWebhookSinkDrainsBodyOnErrorStatus(t *testing.T) {
	t.Parallel()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		// Return a large error body to make undrained-body bugs obvious.
		body := strings.Repeat("e", 2048)
		w.Header().Set("Content-Length", "2048")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	sink, err := NewWebhookSink(WebhookOptions{Endpoint: server.URL, Client: server.Client(), })
	if err != nil {
		t.Fatalf("NewWebhookSink: %v", err)
	}

	// Two requests through the same client; if the body is not drained the
	// second request will fail or open a new connection instead of reusing.
	for i := 0; i < 2; i++ {
		err := sink.EnqueuePush(context.Background(), Notification{MessageID: "msg-1", UserID: "user-1"})
		if err == nil {
			t.Fatalf("request %d: expected error for HTTP 500, got nil", i+1)
		}
		if !strings.Contains(err.Error(), "HTTP 500") {
			t.Fatalf("request %d: error = %v, want HTTP 500", i+1, err)
		}
	}

	if requestCount != 2 {
		t.Fatalf("server received %d requests, want 2 (connection reuse failed)", requestCount)
	}
}
