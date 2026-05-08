package pushnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// HTTPClient abstracts HTTP requests for testability.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Payload is a push notification payload.
type Payload struct {
	Title string
	Body  string
	Data  map[string]string
}

// JSON serializes the payload to JSON.
func (p *Payload) JSON() ([]byte, error) {
	return json.Marshal(p)
}

// PushSink sends push notifications to a device token.
type PushSink interface {
	Send(ctx context.Context, deviceToken string, payload *Payload) error
}

// FCMAdapter sends notifications via Firebase Cloud Messaging HTTP v1.
type FCMAdapter struct {
	projectID string
	authToken string
	client    HTTPClient
}

// NewFCMAdapter creates an FCM adapter.
func NewFCMAdapter(projectID, authToken string, client HTTPClient) *FCMAdapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &FCMAdapter{
		projectID: projectID,
		authToken: authToken,
		client:    client,
	}
}

// Send delivers a push notification via FCM.
func (f *FCMAdapter) Send(ctx context.Context, deviceToken string, payload *Payload) error {
	body := map[string]interface{}{
		"message": map[string]interface{}{
			"token": deviceToken,
			"notification": map[string]string{
				"title": payload.Title,
				"body":  payload.Body,
			},
			"data": payload.Data,
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("fcm marshal: %w", err)
	}

	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", f.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("fcm request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+f.authToken)

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("fcm send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("fcm error %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// APNsAdapter sends notifications via Apple Push Notification service.
type APNsAdapter struct {
	bundleID string
	authKey  string
	client   HTTPClient
}

// NewAPNsAdapter creates an APNs adapter.
func NewAPNsAdapter(bundleID, authKey string, client HTTPClient) *APNsAdapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &APNsAdapter{
		bundleID: bundleID,
		authKey:  authKey,
		client:   client,
	}
}

// Send delivers a push notification via APNs.
func (a *APNsAdapter) Send(ctx context.Context, deviceToken string, payload *Payload) error {
	body := map[string]interface{}{
		"aps": map[string]interface{}{
			"alert": map[string]string{
				"title": payload.Title,
				"body":  payload.Body,
			},
		},
		"data": payload.Data,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("apns marshal: %w", err)
	}

	url := fmt.Sprintf("https://api.push.apple.com/3/device/%s", deviceToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("apns request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("apns-topic", a.bundleID)
	req.Header.Set("authorization", "bearer "+a.authKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("apns send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("apns error %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// WebPushAdapter sends notifications via Web Push Protocol (RFC 8030).
type WebPushAdapter struct {
	vapidSubject string
	vapidKey     string
	client       HTTPClient
}

// NewWebPushAdapter creates a Web Push adapter.
func NewWebPushAdapter(vapidSubject, vapidKey string, client HTTPClient) *WebPushAdapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &WebPushAdapter{
		vapidSubject: vapidSubject,
		vapidKey:     vapidKey,
		client:       client,
	}
}

// Send delivers a push notification via Web Push.
func (w *WebPushAdapter) Send(ctx context.Context, endpoint string, payload *Payload) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webpush marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("webpush request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("ttl", "60")
	req.Header.Set("urgency", "normal")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webpush send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webpush error %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// MultiSink sends to multiple sinks.
type MultiSink struct {
	sinks []PushSink
}

// NewMultiSink creates a sink that broadcasts to all provided sinks.
func NewMultiSink(sinks ...PushSink) *MultiSink {
	return &MultiSink{sinks: sinks}
}

// Send delivers the payload to all sinks.
func (m *MultiSink) Send(ctx context.Context, deviceToken string, payload *Payload) error {
	for _, sink := range m.sinks {
		if err := sink.Send(ctx, deviceToken, payload); err != nil {
			return err
		}
	}
	return nil
}

// DeviceToken represents a registered push notification token.
type DeviceToken struct {
	Token     string
	Platform  string
	AppBundle string
	CreatedAt time.Time
}

// DeviceTokenStore persists device tokens.
type DeviceTokenStore interface {
	Save(ctx context.Context, token *DeviceToken) error
	Delete(ctx context.Context, token string) error
	GetByPlatform(ctx context.Context, platform string) ([]*DeviceToken, error)
}

// MemoryDeviceTokenStore is an in-memory implementation for testing.
type MemoryDeviceTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*DeviceToken
}

// NewMemoryDeviceTokenStore creates an in-memory token store.
func NewMemoryDeviceTokenStore() *MemoryDeviceTokenStore {
	return &MemoryDeviceTokenStore{
		tokens: make(map[string]*DeviceToken),
	}
}

// Save stores a device token.
func (m *MemoryDeviceTokenStore) Save(ctx context.Context, token *DeviceToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[token.Token] = token
	return nil
}

// Delete removes a device token.
func (m *MemoryDeviceTokenStore) Delete(ctx context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, token)
	return nil
}

// GetByPlatform returns tokens for a given platform.
func (m *MemoryDeviceTokenStore) GetByPlatform(ctx context.Context, platform string) ([]*DeviceToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*DeviceToken
	for _, t := range m.tokens {
		if t.Platform == platform {
			result = append(result, t)
		}
	}
	return result, nil
}
