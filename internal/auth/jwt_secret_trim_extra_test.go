package auth

import "testing"

func TestNewTokenManagerRejectsWhitespaceSecret(t *testing.T) {
	if _, err := NewTokenManager(" \t\n "); err == nil {
		t.Fatal("NewTokenManager accepted a whitespace-only secret")
	}
}
