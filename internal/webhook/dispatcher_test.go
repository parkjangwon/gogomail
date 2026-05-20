package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockConfigLoader struct {
	webhooks []WebhookEntry
	err      error
}

func (m *mockConfigLoader) LoadWebhooks(_ context.Context, _ string) ([]WebhookEntry, error) {
	return m.webhooks, m.err
}

func TestWebhookDispatcherDelivers(t *testing.T) {
	var receivedBody map[string]interface{}
	var receivedEvent string
	var receivedDelivery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedEvent = r.Header.Get("X-Gogomail-Event")
		receivedDelivery = r.Header.Get("X-Gogomail-Delivery")
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	loader := &mockConfigLoader{
		webhooks: []WebhookEntry{
			{ID: "wh-1", URL: server.URL, Events: []string{"mail.received"}, Enabled: true},
		},
	}
	dispatcher := NewWebhookDispatcher(WebhookDispatcherOptions{
		Loader:       loader,
		Client:       server.Client(),
		URLGuardOpts: OutboundURLGuardOptions{AllowPrivateNetwork: true},
	})

	err := dispatcher.Dispatch(context.Background(), WebhookEvent{
		Event:      "mail.received",
		CompanyID:  "company-1",
		OccurredAt: "2026-01-01T00:00:00Z",
		Data:       map[string]interface{}{"message_id": "msg-1"},
	})
	if err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	if receivedEvent != "mail.received" {
		t.Errorf("X-Gogomail-Event = %q, want mail.received", receivedEvent)
	}
	if receivedDelivery == "" {
		t.Errorf("X-Gogomail-Delivery header is empty")
	}
	if receivedBody["event"] != "mail.received" {
		t.Errorf("body event = %v, want mail.received", receivedBody["event"])
	}
}

func TestWebhookDispatcherSkipsDisabled(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	loader := &mockConfigLoader{
		webhooks: []WebhookEntry{
			{ID: "wh-1", URL: server.URL, Events: []string{"mail.received"}, Enabled: false},
		},
	}
	dispatcher := NewWebhookDispatcher(WebhookDispatcherOptions{
		Loader:       loader,
		Client:       server.Client(),
		URLGuardOpts: OutboundURLGuardOptions{AllowPrivateNetwork: true},
	})
	_ = dispatcher.Dispatch(context.Background(), WebhookEvent{
		Event:     "mail.received",
		CompanyID: "company-1",
	})
	if called {
		t.Errorf("disabled webhook should not be called")
	}
}

func TestWebhookDispatcherSkipsNonSubscribedEvent(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	loader := &mockConfigLoader{
		webhooks: []WebhookEntry{
			{ID: "wh-1", URL: server.URL, Events: []string{"mail.bounced"}, Enabled: true},
		},
	}
	dispatcher := NewWebhookDispatcher(WebhookDispatcherOptions{
		Loader:       loader,
		Client:       server.Client(),
		URLGuardOpts: OutboundURLGuardOptions{AllowPrivateNetwork: true},
	})
	_ = dispatcher.Dispatch(context.Background(), WebhookEvent{
		Event:     "mail.received",
		CompanyID: "company-1",
	})
	if called {
		t.Errorf("webhook should not be called for unsubscribed event")
	}
}

func TestWebhookDispatcherWildcardEvent(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	loader := &mockConfigLoader{
		webhooks: []WebhookEntry{
			{ID: "wh-1", URL: server.URL, Events: []string{"*"}, Enabled: true},
		},
	}
	dispatcher := NewWebhookDispatcher(WebhookDispatcherOptions{
		Loader:       loader,
		Client:       server.Client(),
		URLGuardOpts: OutboundURLGuardOptions{AllowPrivateNetwork: true},
	})
	_ = dispatcher.Dispatch(context.Background(), WebhookEvent{
		Event:     "mail.received",
		CompanyID: "company-1",
	})
	if !called {
		t.Errorf("wildcard webhook should be called for any event")
	}
}

func TestWebhookDispatcherHMACSignature(t *testing.T) {
	var gotSig string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Gogomail-Signature")
		var buf strings.Builder
		for {
			b := make([]byte, 256)
			n, _ := r.Body.Read(b)
			if n == 0 {
				break
			}
			buf.Write(b[:n])
		}
		gotBody = []byte(buf.String())
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	loader := &mockConfigLoader{
		webhooks: []WebhookEntry{
			{ID: "wh-1", URL: server.URL, Events: []string{"mail.received"}, Enabled: true, Secret: "test-secret"},
		},
	}
	dispatcher := NewWebhookDispatcher(WebhookDispatcherOptions{
		Loader:       loader,
		Client:       server.Client(),
		URLGuardOpts: OutboundURLGuardOptions{AllowPrivateNetwork: true},
	})
	_ = dispatcher.Dispatch(context.Background(), WebhookEvent{
		Event:     "mail.received",
		CompanyID: "company-1",
	})
	if !strings.HasPrefix(gotSig, "sha256=") {
		t.Errorf("signature header = %q, want sha256=...", gotSig)
	}
	_ = gotBody
}

func TestWebhookDispatcherHTTPFailureLogged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer server.Close()

	loader := &mockConfigLoader{
		webhooks: []WebhookEntry{
			{ID: "wh-1", URL: server.URL, Events: []string{"mail.received"}, Enabled: true},
		},
	}
	dispatcher := NewWebhookDispatcher(WebhookDispatcherOptions{
		Loader:       loader,
		Client:       server.Client(),
		URLGuardOpts: OutboundURLGuardOptions{AllowPrivateNetwork: true},
	})
	// Dispatch should not return error (best-effort), just log.
	err := dispatcher.Dispatch(context.Background(), WebhookEvent{
		Event:     "mail.received",
		CompanyID: "company-1",
	})
	if err != nil {
		t.Errorf("Dispatch returned error on HTTP failure (should be best-effort): %v", err)
	}
}

func TestWebhookDispatcherNoWebhooks(t *testing.T) {
	loader := &mockConfigLoader{webhooks: nil}
	dispatcher := NewWebhookDispatcher(WebhookDispatcherOptions{
		Loader: loader,
	})
	err := dispatcher.Dispatch(context.Background(), WebhookEvent{
		Event:     "mail.received",
		CompanyID: "company-1",
	})
	if err != nil {
		t.Errorf("Dispatch returned error with no webhooks: %v", err)
	}
}
