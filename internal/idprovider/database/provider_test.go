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

func TestCreateUserValidation(t *testing.T) {
	provider := New(nil, nil)

	// Test invalid user (nil)
	err := provider.CreateUser(context.Background(), nil)
	if err == nil {
		t.Errorf("Expected error for nil user, got nil")
	}

	// Test missing domain_id
	err = provider.CreateUser(context.Background(), &idprovider.User{
		Username:    "testuser",
		DisplayName: "Test User",
	})
	if err == nil {
		t.Errorf("Expected error for missing domain_id, got nil")
	}

	// Test missing username
	err = provider.CreateUser(context.Background(), &idprovider.User{
		DomainID:    "domain-id",
		DisplayName: "Test User",
	})
	if err == nil {
		t.Errorf("Expected error for missing username, got nil")
	}
}

func TestUpdateUserValidation(t *testing.T) {
	provider := New(nil, nil)

	// Test invalid user (nil)
	err := provider.UpdateUser(context.Background(), nil)
	if err == nil {
		t.Errorf("Expected error for nil user, got nil")
	}

	// Test missing user id
	err = provider.UpdateUser(context.Background(), &idprovider.User{})
	if err == nil {
		t.Errorf("Expected error for missing user id, got nil")
	}
}

func TestDeleteUserValidation(t *testing.T) {
	provider := New(nil, nil)

	// Test empty user id
	err := provider.DeleteUser(context.Background(), "")
	if err == nil {
		t.Errorf("Expected error for empty user id, got nil")
	}
}
