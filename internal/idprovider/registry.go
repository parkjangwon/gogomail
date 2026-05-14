package idprovider

import (
	"fmt"
	"sync"
)

var (
	providersMu sync.RWMutex
	providers   = make(map[string]IdentityProvider)
)

// Register registers an identity provider by type.
func Register(providerType string, provider IdentityProvider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[providerType] = provider
}

// Get retrieves a registered identity provider by type.
// Returns an error if the provider type is not registered.
func Get(providerType string) (IdentityProvider, error) {
	providersMu.RLock()
	defer providersMu.RUnlock()

	provider, ok := providers[providerType]
	if !ok {
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
	return provider, nil
}
