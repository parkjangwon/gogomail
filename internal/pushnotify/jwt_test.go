package pushnotify

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestGenerateAPNsJWT(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	token, err := generateAPNsJWT("TESTKID123", "TESTTEAM1", key, time.Now())
	if err != nil {
		t.Fatalf("generateAPNsJWT: %v", err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d: %s", len(parts), token)
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if !strings.Contains(string(headerJSON), `"ES256"`) {
		t.Errorf("header missing alg ES256: %s", headerJSON)
	}
	if !strings.Contains(string(headerJSON), "TESTKID123") {
		t.Errorf("header missing kid: %s", headerJSON)
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if !strings.Contains(string(payloadJSON), "TESTTEAM1") {
		t.Errorf("payload missing iss: %s", payloadJSON)
	}
	if !strings.Contains(string(payloadJSON), "iat") {
		t.Errorf("payload missing iat: %s", payloadJSON)
	}
}

func TestAPNsJWTCaching(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	adapter := NewAPNsAdapterFromKey(APNsConfig{
		BundleID: "com.test.app",
		KeyID:    "KID1",
		TeamID:   "TEAM1",
	}, key, nil)

	tok1, err := adapter.jwt()
	if err != nil {
		t.Fatalf("first jwt: %v", err)
	}
	tok2, err := adapter.jwt()
	if err != nil {
		t.Fatalf("second jwt: %v", err)
	}
	if tok1 != tok2 {
		t.Errorf("expected cached JWT, got different tokens")
	}
}

func TestGenerateVAPIDJWT(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	token, err := generateVAPIDJWT("https://fcm.googleapis.com/fcm/send/abc", "test@example.com", key, time.Now())
	if err != nil {
		t.Fatalf("generateVAPIDJWT: %v", err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if !strings.Contains(string(headerJSON), "ES256") {
		t.Errorf("header missing ES256: %s", headerJSON)
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if !strings.Contains(string(payloadJSON), "fcm.googleapis.com") {
		t.Errorf("payload missing aud: %s", payloadJSON)
	}
	if !strings.Contains(string(payloadJSON), "test@example.com") {
		t.Errorf("payload missing sub: %s", payloadJSON)
	}
}

func TestParseECPrivateKey(t *testing.T) {
	if _, err := parseECPrivateKey("not a pem"); err == nil {
		t.Fatalf("expected error for non-PEM input")
	}
}

func TestParseVAPIDPrivateKey(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	scalar := make([]byte, 32)
	key.D.FillBytes(scalar)
	encoded := base64.RawURLEncoding.EncodeToString(scalar)

	parsed, err := parseVAPIDPrivateKey(encoded)
	if err != nil {
		t.Fatalf("parseVAPIDPrivateKey: %v", err)
	}
	if parsed == nil {
		t.Fatalf("expected non-nil key")
	}
	if _, err := parseVAPIDPrivateKey("not-base64!!!"); err == nil {
		t.Fatalf("expected error for invalid base64")
	}
	if _, err := parseVAPIDPrivateKey(""); err == nil {
		t.Fatalf("expected error for empty key")
	}
}
