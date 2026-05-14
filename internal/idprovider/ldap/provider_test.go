package ldap

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
	provider := New(&Config{
		Host:      "ldap.example.com",
		Port:      389,
		BaseDN:    "dc=example,dc=com",
		BindDN:    "cn=admin,dc=example,dc=com",
		BindPass:  "password",
		UsersDN:   "ou=users,dc=example,dc=com",
		GroupsDN:  "ou=groups,dc=example,dc=com",
		UserAttr:  "uid",
		GroupAttr: "cn",
	})
	if provider == nil {
		t.Errorf("Expected non-nil provider")
	}
}

func TestGetUserValidation(t *testing.T) {
	provider := New(nil)

	// Test empty user ID
	_, err := provider.GetUser(context.Background(), "")
	if err == nil {
		t.Errorf("Expected error for empty user id, got nil")
	}
}

func TestListUsersValidation(t *testing.T) {
	provider := New(nil)

	// Test listing users with nil config (should not panic)
	users, err := provider.ListUsers(context.Background(), nil)
	if err != nil {
		// Error is expected when no config provided
		if users != nil {
			t.Errorf("Expected nil users on error")
		}
	}
}

func TestGetGroupValidation(t *testing.T) {
	provider := New(nil)

	// Test empty group ID
	_, err := provider.GetGroup(context.Background(), "")
	if err == nil {
		t.Errorf("Expected error for empty group id, got nil")
	}
}

func TestCreateUserValidation(t *testing.T) {
	provider := New(nil)

	// Test nil user
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
}

func TestUpdateUserValidation(t *testing.T) {
	provider := New(nil)

	// Test nil user
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
	provider := New(nil)

	// Test empty user ID
	err := provider.DeleteUser(context.Background(), "")
	if err == nil {
		t.Errorf("Expected error for empty user id, got nil")
	}
}

func TestCreateGroupValidation(t *testing.T) {
	provider := New(nil)

	// Test nil group
	err := provider.CreateGroup(context.Background(), nil)
	if err == nil {
		t.Errorf("Expected error for nil group, got nil")
	}

	// Test missing domain_id
	err = provider.CreateGroup(context.Background(), &idprovider.Group{
		Name: "test-group",
	})
	if err == nil {
		t.Errorf("Expected error for missing domain_id, got nil")
	}
}

func TestDeleteGroupValidation(t *testing.T) {
	provider := New(nil)

	// Test empty group ID
	err := provider.DeleteGroup(context.Background(), "")
	if err == nil {
		t.Errorf("Expected error for empty group id, got nil")
	}
}

func TestAddMemberValidation(t *testing.T) {
	provider := New(nil)

	// Test empty group ID
	err := provider.AddMember(context.Background(), "", nil)
	if err == nil {
		t.Errorf("Expected error for empty group id, got nil")
	}

	// Test nil member
	err = provider.AddMember(context.Background(), "group-id", nil)
	if err == nil {
		t.Errorf("Expected error for nil member, got nil")
	}
}

func TestRemoveMemberValidation(t *testing.T) {
	provider := New(nil)

	// Test empty group ID
	err := provider.RemoveMember(context.Background(), "", "member-id")
	if err == nil {
		t.Errorf("Expected error for empty group id, got nil")
	}

	// Test empty member ID
	err = provider.RemoveMember(context.Background(), "group-id", "")
	if err == nil {
		t.Errorf("Expected error for empty member id, got nil")
	}
}
