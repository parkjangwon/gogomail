package database

import (
	"context"
	"testing"

	"github.com/gogomail/gogomail/internal/idprovider"
)

func TestProviderInterface(t *testing.T) {
	// Verify that Provider implements IdentityProvider
	var _ idprovider.IdentityProvider = (*Provider)(nil)
}

func TestNewProvider(t *testing.T) {
	provider := New(nil, nil)
	if provider == nil {
		t.Errorf("Expected non-nil provider")
	}
}

func TestProviderGetUserNotSupported(t *testing.T) {
	provider := New(nil, nil)

	// Test that CreateUser returns an error (unsupported for DB provider)
	err := provider.CreateUser(context.Background(), &idprovider.User{})
	if err == nil {
		t.Errorf("Expected error for CreateUser, got nil")
	}

	// Test that UpdateUser returns an error
	err = provider.UpdateUser(context.Background(), &idprovider.User{})
	if err == nil {
		t.Errorf("Expected error for UpdateUser, got nil")
	}

	// Test that DeleteUser returns an error
	err = provider.DeleteUser(context.Background(), "user-id")
	if err == nil {
		t.Errorf("Expected error for DeleteUser, got nil")
	}
}
