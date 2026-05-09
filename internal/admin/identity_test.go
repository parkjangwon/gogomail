package admin

import (
	"context"
	"fmt"
	"testing"
)

// IdentityProvider defines the interface for identity backends
type IdentityProvider interface {
	// Authenticate validates credentials and returns user info
	Authenticate(ctx context.Context, credentials map[string]string) (*ProviderUser, error)

	// GetUser retrieves a user by ID
	GetUser(ctx context.Context, userID string) (*ProviderUser, error)

	// ListUsers lists users with filtering
	ListUsers(ctx context.Context, filter map[string]string, limit, offset int) ([]*ProviderUser, int64, error)

	// SyncUsers performs a full or incremental sync from the provider
	SyncUsers(ctx context.Context, incremental bool) (*SyncResult, error)

	// Validate checks if provider configuration is valid
	Validate(ctx context.Context) error
}

// ProviderUser represents a user from an identity provider
type ProviderUser struct {
	ExternalID string            `json:"external_id"` // ID in the external system
	Email      string            `json:"email"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"` // custom attributes
}

// SyncResult contains results of a user sync operation
type SyncResult struct {
	Created   int    `json:"created"`
	Updated   int    `json:"updated"`
	Deleted   int    `json:"deleted"`
	Failed    int    `json:"failed"`
	Duration  int64  `json:"duration_ms"` // milliseconds
	Errors    []string `json:"errors,omitempty"`
	LastToken string `json:"last_token,omitempty"` // for incremental sync
}

// ProviderType represents the type of identity provider
type ProviderType string

const (
	ProviderTypeDatabase = "database"
	ProviderTypeLDAP     = "ldap"
	ProviderTypeAzureAD  = "azure_ad"
	ProviderTypeRDBMS    = "external_rdbms"
)

// ProviderRegistry manages available identity providers
type ProviderRegistry struct {
	providers map[ProviderType]IdentityProvider
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[ProviderType]IdentityProvider),
	}
}

// Register registers a provider
func (pr *ProviderRegistry) Register(providerType ProviderType, provider IdentityProvider) error {
	if string(providerType) == "" {
		return fmt.Errorf("%w: providerType", ErrMissingRequiredField)
	}
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}
	pr.providers[providerType] = provider
	return nil
}

// Get retrieves a registered provider
func (pr *ProviderRegistry) Get(providerType ProviderType) (IdentityProvider, error) {
	provider, ok := pr.providers[providerType]
	if !ok {
		return nil, fmt.Errorf("provider not registered: %s", providerType)
	}
	return provider, nil
}

// List returns all registered provider types
func (pr *ProviderRegistry) List() []ProviderType {
	var types []ProviderType
	for t := range pr.providers {
		types = append(types, t)
	}
	return types
}

// Mock provider for testing
type mockIdentityProvider struct {
	users map[string]*ProviderUser
}

func newMockIdentityProvider() *mockIdentityProvider {
	return &mockIdentityProvider{
		users: make(map[string]*ProviderUser),
	}
}

func (m *mockIdentityProvider) Authenticate(ctx context.Context, credentials map[string]string) (*ProviderUser, error) {
	email, ok := credentials["email"]
	if !ok {
		return nil, fmt.Errorf("email required")
	}
	password, ok := credentials["password"]
	if !ok {
		return nil, fmt.Errorf("password required")
	}
	if password == "" {
		return nil, fmt.Errorf("invalid credentials")
	}
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (m *mockIdentityProvider) GetUser(ctx context.Context, userID string) (*ProviderUser, error) {
	for _, user := range m.users {
		if user.ExternalID == userID {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (m *mockIdentityProvider) ListUsers(ctx context.Context, filter map[string]string, limit, offset int) ([]*ProviderUser, int64, error) {
	var users []*ProviderUser
	for _, user := range m.users {
		users = append(users, user)
	}
	return users, int64(len(users)), nil
}

func (m *mockIdentityProvider) SyncUsers(ctx context.Context, incremental bool) (*SyncResult, error) {
	return &SyncResult{
		Created: 0,
		Updated: 0,
		Deleted: 0,
		Failed:  0,
	}, nil
}

func (m *mockIdentityProvider) Validate(ctx context.Context) error {
	return nil
}

var ErrProviderNotConfigured = fmt.Errorf("provider not configured")

func TestProviderRegistry(t *testing.T) {
	registry := NewProviderRegistry()

	tests := []struct {
		name      string
		action    func() error
		shouldErr bool
	}{
		{
			name: "register provider",
			action: func() error {
				provider := newMockIdentityProvider()
				return registry.Register(ProviderTypeDatabase, provider)
			},
			shouldErr: false,
		},
		{
			name: "register with nil provider",
			action: func() error {
				return registry.Register(ProviderTypeDatabase, nil)
			},
			shouldErr: true,
		},
		{
			name: "register with empty type",
			action: func() error {
				provider := newMockIdentityProvider()
				return registry.Register("", provider)
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.action()
			if (err != nil) != tt.shouldErr {
				t.Errorf("action error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestProviderAuthenticate(t *testing.T) {
	provider := newMockIdentityProvider()
	provider.users["ext-user-1"] = &ProviderUser{
		ExternalID: "ext-user-1",
		Email:      "user@example.com",
		Name:       "Test User",
	}
	ctx := context.Background()

	tests := []struct {
		name      string
		creds     map[string]string
		shouldErr bool
	}{
		{
			name: "valid credentials",
			creds: map[string]string{
				"email":    "user@example.com",
				"password": "password",
			},
			shouldErr: false,
		},
		{
			name: "missing email",
			creds: map[string]string{
				"password": "password",
			},
			shouldErr: true,
		},
		{
			name: "missing password",
			creds: map[string]string{
				"email": "user@example.com",
			},
			shouldErr: true,
		},
		{
			name: "invalid credentials",
			creds: map[string]string{
				"email":    "user@example.com",
				"password": "",
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := provider.Authenticate(ctx, tt.creds)
			if (err != nil) != tt.shouldErr {
				t.Errorf("Authenticate() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && user == nil {
				t.Error("Authenticate() returned nil user")
			}
		})
	}
}
