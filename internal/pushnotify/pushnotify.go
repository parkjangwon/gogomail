package pushnotify

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
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

	fcmURL := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", f.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fcmURL, bytes.NewReader(b))
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

// APNsConfig holds configuration for APNs JWT token-based authentication.
type APNsConfig struct {
	// BundleID is the app bundle identifier used as the apns-topic header.
	BundleID string
	// KeyID is the 10-character key ID from the Apple Developer portal.
	KeyID string
	// TeamID is the 10-character team ID from the Apple Developer portal.
	TeamID string
	// PrivateKeyPEM is a PEM-encoded ECDSA P-256 private key (PKCS8 or SEC1/EC).
	PrivateKeyPEM string
}

// APNsAdapter sends notifications via Apple Push Notification service using ES256 JWT auth.
type APNsAdapter struct {
	bundleID   string
	keyID      string
	teamID     string
	privateKey *ecdsa.PrivateKey
	client     HTTPClient

	mu          sync.Mutex
	cachedJWT   string
	jwtIssuedAt time.Time
}

// apnsJWTMaxAge is the time after which a cached JWT is regenerated.
// Apple allows up to 1 hour; we refresh at 45 minutes.
const apnsJWTMaxAge = 45 * time.Minute

// NewAPNsAdapter creates an APNs adapter using JWT token-based auth (ES256).
func NewAPNsAdapter(cfg APNsConfig, client HTTPClient) (*APNsAdapter, error) {
	if client == nil {
		client = http.DefaultClient
	}
	key, err := parseECPrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("apns private key: %w", err)
	}
	return &APNsAdapter{
		bundleID:   cfg.BundleID,
		keyID:      cfg.KeyID,
		teamID:     cfg.TeamID,
		privateKey: key,
		client:     client,
	}, nil
}

// NewAPNsAdapterFromKey creates an APNs adapter from an already-parsed ECDSA key (for tests).
func NewAPNsAdapterFromKey(cfg APNsConfig, key *ecdsa.PrivateKey, client HTTPClient) *APNsAdapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &APNsAdapter{
		bundleID:   cfg.BundleID,
		keyID:      cfg.KeyID,
		teamID:     cfg.TeamID,
		privateKey: key,
		client:     client,
	}
}

func (a *APNsAdapter) jwt() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cachedJWT != "" && time.Since(a.jwtIssuedAt) < apnsJWTMaxAge {
		return a.cachedJWT, nil
	}
	token, err := generateAPNsJWT(a.keyID, a.teamID, a.privateKey, time.Now())
	if err != nil {
		return "", err
	}
	a.cachedJWT = token
	a.jwtIssuedAt = time.Now()
	return token, nil
}

func generateAPNsJWT(keyID, teamID string, key *ecdsa.PrivateKey, now time.Time) (string, error) {
	header := base64urlJSON(map[string]string{"alg": "ES256", "kid": keyID})
	payload := base64urlJSON(map[string]interface{}{"iss": teamID, "iat": now.Unix()})
	signingInput := header + "." + payload
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", fmt.Errorf("apns jwt sign: %w", err)
	}
	// IEEE P1363 format: R || S, each zero-padded to 32 bytes for P-256.
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// Send delivers a push notification via APNs using ES256 JWT authentication.
func (a *APNsAdapter) Send(ctx context.Context, deviceToken string, payload *Payload) error {
	body := map[string]interface{}{
		"aps": map[string]interface{}{
			"alert": map[string]string{"title": payload.Title, "body": payload.Body},
		},
		"data": payload.Data,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("apns marshal: %w", err)
	}
	token, err := a.jwt()
	if err != nil {
		return fmt.Errorf("apns jwt: %w", err)
	}
	apnsURL := fmt.Sprintf("https://api.push.apple.com/3/device/%s", deviceToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apnsURL, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("apns request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("apns-topic", a.bundleID)
	req.Header.Set("authorization", "bearer "+token)
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

// VAPIDConfig holds configuration for VAPID (RFC 8292) web push authentication.
type VAPIDConfig struct {
	// PublicKey is the VAPID public key, base64url-encoded uncompressed P-256 point (65 bytes).
	PublicKey string
	// PrivateKey is the VAPID private key scalar, base64url-encoded 32-byte big-endian integer.
	PrivateKey string
	// ContactEmail is used in the VAPID JWT sub claim (with or without "mailto:" prefix).
	ContactEmail string
}

// WebPushAdapter sends notifications via Web Push Protocol (RFC 8030) with VAPID (RFC 8292).
type WebPushAdapter struct {
	vapidPublicKey  string
	vapidPrivateKey *ecdsa.PrivateKey
	contactEmail    string
	client          HTTPClient
}

// NewWebPushAdapter creates a Web Push adapter with VAPID authentication.
func NewWebPushAdapter(cfg VAPIDConfig, client HTTPClient) (*WebPushAdapter, error) {
	if client == nil {
		client = http.DefaultClient
	}
	privKey, err := parseVAPIDPrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("vapid private key: %w", err)
	}
	return &WebPushAdapter{
		vapidPublicKey:  cfg.PublicKey,
		vapidPrivateKey: privKey,
		contactEmail:    cfg.ContactEmail,
		client:          client,
	}, nil
}

// NewWebPushAdapterFromKey creates a Web Push adapter from an already-parsed ECDSA key (for tests).
func NewWebPushAdapterFromKey(cfg VAPIDConfig, key *ecdsa.PrivateKey, client HTTPClient) *WebPushAdapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &WebPushAdapter{
		vapidPublicKey:  cfg.PublicKey,
		vapidPrivateKey: key,
		contactEmail:    cfg.ContactEmail,
		client:          client,
	}
}

func generateVAPIDJWT(endpoint, contactEmail string, key *ecdsa.PrivateKey, now time.Time) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	audience := parsed.Scheme + "://" + parsed.Host
	sub := contactEmail
	if sub != "" && !strings.HasPrefix(sub, "mailto:") {
		sub = "mailto:" + sub
	}
	header := base64urlJSON(map[string]string{"typ": "JWT", "alg": "ES256"})
	payload := base64urlJSON(map[string]interface{}{
		"aud": audience,
		"exp": now.Add(12 * time.Hour).Unix(),
		"sub": sub,
	})
	signingInput := header + "." + payload
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", fmt.Errorf("vapid jwt sign: %w", err)
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// Send delivers a push notification via Web Push with VAPID authentication.
// NOTE: payload encryption (aes128gcm, RFC 8291) is not yet implemented.
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
	if w.vapidPrivateKey != nil {
		jwtToken, err := generateVAPIDJWT(endpoint, w.contactEmail, w.vapidPrivateKey, time.Now())
		if err != nil {
			return fmt.Errorf("webpush vapid jwt: %w", err)
		}
		req.Header.Set("authorization", "vapid t="+jwtToken+",k="+w.vapidPublicKey)
	}
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

func parseECPrivateKey(pemData string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	switch block.Type {
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		ec, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key is not ECDSA")
		}
		return ec, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}
}

func parseVAPIDPrivateKey(encoded string) (*ecdsa.PrivateKey, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, fmt.Errorf("private key is required")
	}
	b, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	curve := elliptic.P256()
	if len(b) != 32 {
		return nil, fmt.Errorf("expected 32-byte scalar, got %d bytes", len(b))
	}
	d := new(big.Int).SetBytes(b)
	priv := new(ecdsa.PrivateKey)
	priv.D = d
	priv.PublicKey.Curve = curve
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(b)
	return priv, nil
}

func base64urlJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(b)
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
