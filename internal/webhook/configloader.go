package webhook

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gogomail/gogomail/internal/configstore"
)

const WebhooksConfigKey = "webhooks_config"

type webhooksConfigSchema struct {
	Webhooks []WebhookEntry `json:"webhooks"`
}

// ConfigStoreLoader loads webhook entries from a configstore.
type ConfigStoreLoader struct {
	store configstore.ConfigStore
}

// NewConfigStoreLoader creates a WebhookConfigLoader backed by configstore.
func NewConfigStoreLoader(store configstore.ConfigStore) *ConfigStoreLoader {
	return &ConfigStoreLoader{store: store}
}

// LoadWebhooks returns all webhook entries for the given company.
// Returns nil (not an error) if no webhooks are configured.
func (l *ConfigStoreLoader) LoadWebhooks(ctx context.Context, companyID string) ([]WebhookEntry, error) {
	entry, err := l.store.Get(ctx, configstore.ScopeCompany, companyID, WebhooksConfigKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var cfg webhooksConfigSchema
	if err := json.Unmarshal(entry.Value, &cfg); err != nil {
		return nil, nil
	}
	return cfg.Webhooks, nil
}
