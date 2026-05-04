package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTokenManagerSignVerify(t *testing.T) {
	t.Parallel()

	manager, err := NewTokenManager("secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	now := time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	token, err := manager.Sign(Claims{UserID: "user-1", DomainID: "domain-1", Role: "user"}, time.Minute)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	claims, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if claims.UserID != "user-1" {
		t.Fatalf("UserID = %q, want user-1", claims.UserID)
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	manager, err := NewTokenManager("secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	now := time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	token, err := manager.Sign(Claims{UserID: "user-1"}, time.Minute)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	manager.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := manager.Verify(token); err == nil {
		t.Fatal("Verify accepted expired token")
	}
}

func TestTokenManagerRejectsUnsupportedJWTHeader(t *testing.T) {
	t.Parallel()

	manager, err := NewTokenManager("secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	now := time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	for _, tc := range []struct {
		name    string
		header  map[string]string
		wantErr string
	}{
		{
			name:    "algorithm",
			header:  map[string]string{"alg": "none", "typ": "JWT"},
			wantErr: "unsupported jwt algorithm",
		},
		{
			name:    "type",
			header:  map[string]string{"alg": "HS256", "typ": "JWS"},
			wantErr: "unsupported jwt type",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			token := signedTestToken(t, manager, tc.header, Claims{UserID: "user-1", Expiry: now.Add(time.Minute).Unix()})
			if _, err := manager.Verify(token); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Verify error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func signedTestToken(t *testing.T, manager *TokenManager, header map[string]string, claims Claims) string {
	t.Helper()

	headerRaw, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadRaw, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerRaw) + "." + base64.RawURLEncoding.EncodeToString(payloadRaw)
	return signingInput + "." + manager.sign(signingInput)
}
