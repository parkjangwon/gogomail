package admin

import (
	"context"
	"fmt"
)

// IdentityService manages identity provider operations
type IdentityService struct {
	registry *ProviderRegistry
}

// NewIdentityService creates a new identity service
func NewIdentityService(registry *ProviderRegistry) *IdentityService {
	return &IdentityService{
		registry: registry,
	}
}

// AuthenticateWithProvider authenticates a user using a specific provider
func (s *IdentityService) AuthenticateWithProvider(ctx context.Context, providerType ProviderType, credentials map[string]string) (*ProviderUser, error) {
	if string(providerType) == "" {
		return nil, fmt.Errorf("%w: providerType", ErrMissingRequiredField)
	}

	provider, err := s.registry.Get(providerType)
	if err != nil {
		return nil, err
	}

	return provider.Authenticate(ctx, credentials)
}

// GetUserFromProvider retrieves a user from a provider
func (s *IdentityService) GetUserFromProvider(ctx context.Context, providerType ProviderType, userID string) (*ProviderUser, error) {
	if string(providerType) == "" {
		return nil, fmt.Errorf("%w: providerType", ErrMissingRequiredField)
	}
	if userID == "" {
		return nil, fmt.Errorf("%w: userID", ErrMissingRequiredField)
	}

	provider, err := s.registry.Get(providerType)
	if err != nil {
		return nil, err
	}

	return provider.GetUser(ctx, userID)
}

// ListUsersFromProvider lists users from a provider
func (s *IdentityService) ListUsersFromProvider(ctx context.Context, providerType ProviderType, filter map[string]string, limit, offset int) ([]*ProviderUser, int64, error) {
	if string(providerType) == "" {
		return nil, 0, fmt.Errorf("%w: providerType", ErrMissingRequiredField)
	}

	provider, err := s.registry.Get(providerType)
	if err != nil {
		return nil, 0, err
	}

	if limit == 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	return provider.ListUsers(ctx, filter, limit, offset)
}

// SyncUsersFromProvider syncs users from a provider
func (s *IdentityService) SyncUsersFromProvider(ctx context.Context, providerType ProviderType, incremental bool) (*SyncResult, error) {
	if string(providerType) == "" {
		return nil, fmt.Errorf("%w: providerType", ErrMissingRequiredField)
	}

	provider, err := s.registry.Get(providerType)
	if err != nil {
		return nil, err
	}

	return provider.SyncUsers(ctx, incremental)
}

// ValidateProvider validates a provider's configuration
func (s *IdentityService) ValidateProvider(ctx context.Context, providerType ProviderType) error {
	if string(providerType) == "" {
		return fmt.Errorf("%w: providerType", ErrMissingRequiredField)
	}

	provider, err := s.registry.Get(providerType)
	if err != nil {
		return err
	}

	return provider.Validate(ctx)
}

// IsProviderConfigured checks if a provider type is registered
func (s *IdentityService) IsProviderConfigured(providerType ProviderType) bool {
	_, err := s.registry.Get(providerType)
	return err == nil
}

// GetConfiguredProviders returns all configured provider types
func (s *IdentityService) GetConfiguredProviders() []ProviderType {
	return s.registry.List()
}
