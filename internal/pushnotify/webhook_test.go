package pushnotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

	sink, err := NewWebhookSink(WebhookOptions{Endpoint: server.URL, Token: " push-token ", Client: server.Client()})
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

func TestWebhookSinkReturnsHTTPFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	sink, err := NewWebhookSink(WebhookOptions{Endpoint: server.URL, Client: server.Client()})
	if err != nil {
		t.Fatalf("NewWebhookSink returned error: %v", err)
	}
	err = sink.EnqueuePush(context.Background(), Notification{MessageID: "msg-1", UserID: "user-1"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("EnqueuePush error = %v, want HTTP 502", err)
	}
}

func TestNewWebhookSinkRejectsUnsupportedEndpoint(t *testing.T) {
	t.Parallel()

	if _, err := NewWebhookSink(WebhookOptions{Endpoint: "ftp://push.example/send"}); err == nil {
		t.Fatal("NewWebhookSink accepted unsupported endpoint")
	}
}
