package pushnotify

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

type mockHTTPClient struct {
	response *http.Response
	err      error
	requests []*http.Request
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func TestFCMAdapterSend(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(`{"name":"projects/test/messages/123"}`)),
		},
	}
	adapter := NewFCMAdapter("test-project", "fake-token", client)

	payload := &Payload{
		Title: "Hello",
		Body:  "World",
		Data:  map[string]string{"key": "value"},
	}
	err := adapter.Send(context.Background(), "device-token-1", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(client.requests))
	}
	req := client.requests[0]
	if req.URL.Path != "/v1/projects/test-project/messages:send" {
		t.Fatalf("unexpected path: %s", req.URL.Path)
	}
}

func TestFCMAdapterSendFailure(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 400,
			Body:       io.NopCloser(bytes.NewBufferString(`{"error":{"message":"Invalid registration"}}`)),
		},
	}
	adapter := NewFCMAdapter("test-project", "fake-token", client)

	payload := &Payload{Title: "Test", Body: "Body"}
	err := adapter.Send(context.Background(), "bad-token", payload)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestAPNsAdapterSend(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(``)),
		},
	}
	adapter := NewAPNsAdapter("com.example.app", "fake-cert", client)

	payload := &Payload{
		Title: "Hello",
		Body:  "APNs",
	}
	err := adapter.Send(context.Background(), "apns-token-1", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(client.requests))
	}
	req := client.requests[0]
	if req.URL.Host != "api.push.apple.com" {
		t.Fatalf("unexpected host: %s", req.URL.Host)
	}
	auth := req.Header.Get("authorization")
	if auth != "bearer fake-cert" {
		t.Fatalf("unexpected authorization: %s", auth)
	}
}

func TestWebPushAdapterSend(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 201,
			Body:       io.NopCloser(bytes.NewBufferString(``)),
		},
	}
	adapter := NewWebPushAdapter("mailto:test@example.com", "vapid-key-pair", client)

	payload := &Payload{
		Title: "Hello",
		Body:  "WebPush",
	}
	err := adapter.Send(context.Background(), "https://fcm.googleapis.com/fcm/send/token", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(client.requests))
	}
	req := client.requests[0]
	ttl := req.Header.Get("ttl")
	if ttl == "" {
		t.Fatalf("expected TTL header")
	}
}

func TestPayloadJSON(t *testing.T) {
	p := &Payload{
		Title: "T",
		Body:  "B",
		Data:  map[string]string{"k": "v"},
	}
	b, err := p.JSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b) == 0 {
		t.Fatalf("expected non-empty JSON")
	}
}

func TestMultiSinkSendAll(t *testing.T) {
	fcmClient := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
		},
	}
	apnsClient := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(``)),
		},
	}

	fcm := NewFCMAdapter("proj", "token", fcmClient)
	apns := NewAPNsAdapter("bundle", "cert", apnsClient)

	multi := NewMultiSink(fcm, apns)
	payload := &Payload{Title: "Multi", Body: "Test"}

	err := multi.Send(context.Background(), "token", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fcmClient.requests) != 1 {
		t.Fatalf("expected FCM request")
	}
	if len(apnsClient.requests) != 1 {
		t.Fatalf("expected APNs request")
	}
}

func TestDeviceTokenStore(t *testing.T) {
	store := NewMemoryDeviceTokenStore()
	token := &DeviceToken{
		Token:     "tok-1",
		Platform:  "ios",
		AppBundle: "com.example.app",
		CreatedAt: time.Now(),
	}

	if err := store.Save(context.Background(), token); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tokens, err := store.GetByPlatform(context.Background(), "ios")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Token != "tok-1" {
		t.Fatalf("unexpected token: %s", tokens[0].Token)
	}

	if err := store.Delete(context.Background(), "tok-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tokens, err = store.GetByPlatform(context.Background(), "ios")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens after delete, got %d", len(tokens))
	}
}

func TestFCMAdapterNetworkError(t *testing.T) {
	client := &mockHTTPClient{err: errors.New("connection refused")}
	adapter := NewFCMAdapter("proj", "token", client)

	err := adapter.Send(context.Background(), "tok", &Payload{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestAPNsAdapterBadToken(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 410,
			Body:       io.NopCloser(bytes.NewBufferString(`{"reason":"Unregistered"}`)),
		},
	}
	adapter := NewAPNsAdapter("bundle", "cert", client)

	err := adapter.Send(context.Background(), "tok", &Payload{})
	if err == nil {
		t.Fatalf("expected error for 410 Gone")
	}
}
