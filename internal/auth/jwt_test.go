package auth

import (
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
