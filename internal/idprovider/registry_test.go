package idprovider

import (
	"context"
	"testing"
)

func TestRegistry(t *testing.T) {
	// Clear registry for test isolation
	providersMu.Lock()
	providers = make(map[string]IdentityProvider)
	providersMu.Unlock()

	// Test registering a provider (reuse mockProvider from interface_test.go)
	testProvider := &testMockProvider{}
	Register("test", testProvider)

	// Test retrieving registered provider
	provider, err := Get("test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if provider != testProvider {
		t.Errorf("Expected registered provider, got different instance")
	}

	// Test retrieving unregistered provider
	_, err = Get("unknown")
	if err == nil {
		t.Errorf("Expected error for unknown provider, got nil")
	}
}

type testMockProvider struct{}

func (m *testMockProvider) GetUser(ctx context.Context, userID string) (*User, error) {
	return nil, nil
}

func (m *testMockProvider) GetGroup(ctx context.Context, groupID string) (*Group, error) {
	return nil, nil
}

func (m *testMockProvider) ListUsers(ctx context.Context, filter *UserFilter) ([]*User, error) {
	return nil, nil
}

func (m *testMockProvider) ListGroups(ctx context.Context, filter *GroupFilter) ([]*Group, error) {
	return nil, nil
}

func (m *testMockProvider) CreateUser(ctx context.Context, user *User) error {
	return nil
}

func (m *testMockProvider) UpdateUser(ctx context.Context, user *User) error {
	return nil
}

func (m *testMockProvider) DeleteUser(ctx context.Context, userID string) error {
	return nil
}

func (m *testMockProvider) CreateGroup(ctx context.Context, group *Group) error {
	return nil
}

func (m *testMockProvider) DeleteGroup(ctx context.Context, groupID string) error {
	return nil
}

func (m *testMockProvider) AddMember(ctx context.Context, groupID string, member *Member) error {
	return nil
}

func (m *testMockProvider) RemoveMember(ctx context.Context, groupID, memberID string) error {
	return nil
}
