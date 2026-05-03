package auth

import (
	"testing"
	"time"
)

func TestTokenManagerSignUsesDefaultTTLWhenNonPositive(t *testing.T) {
	manager, err := NewTokenManager("secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	now := time.Date(2026, 5, 3, 1, 2, 3, 0, time.UTC)
	manager.now = func() time.Time { return now }
	token, err := manager.Sign(Claims{UserID: "user-1"}, 0)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	claims, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if claims.Expires.Sub(now) != 15*time.Minute {
		t.Fatalf("default ttl = %s, want 15m", claims.Expires.Sub(now))
	}
}
