package idprovider

import (
	"errors"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Config represents per-domain identity provider configuration.
type Config struct {
	DomainID     string                 `json:"domain_id"`
	ProviderType string                 `json:"provider_type"`
	Settings     map[string]interface{} `json:"settings"`
}

// ConfigRepository handles IdP configuration persistence.
type ConfigRepository struct {
	db *sql.DB
}

// NewConfigRepository creates a new configuration repository.
func NewConfigRepository(db *sql.DB) *ConfigRepository {
	return &ConfigRepository{db: db}
}

// GetConfigByDomain retrieves the IdP configuration for a domain, with fallback to database mode.
func (r *ConfigRepository) GetConfigByDomain(ctx context.Context, domainID string) (*Config, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT provider_type, config
		FROM idp_configurations
		WHERE domain_id = $1 AND status = 'active'
		LIMIT 1
	`, domainID)

	var providerType string
	var configJSON json.RawMessage

	err := row.Scan(&providerType, &configJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Fallback to database mode if no config exists
			return &Config{
				DomainID:     domainID,
				ProviderType: "database",
				Settings:     make(map[string]interface{}),
			}, nil
		}
		return nil, fmt.Errorf("failed to query IdP configuration: %w", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(configJSON, &settings); err != nil {
		settings = make(map[string]interface{})
	}

	return &Config{
		DomainID:     domainID,
		ProviderType: providerType,
		Settings:     settings,
	}, nil
}

// CreateConfig creates a new IdP configuration for a domain.
func (r *ConfigRepository) CreateConfig(ctx context.Context, config *Config) error {
	if config == nil || config.DomainID == "" || config.ProviderType == "" {
		return fmt.Errorf("invalid config: missing required fields")
	}

	settings, _ := json.Marshal(config.Settings)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO idp_configurations (domain_id, provider_type, config)
		VALUES ($1, $2, $3)
		ON CONFLICT (domain_id) WHERE status = 'active' DO NOTHING
	`, config.DomainID, config.ProviderType, settings)

	return err
}

// UpdateConfig updates an existing IdP configuration.
func (r *ConfigRepository) UpdateConfig(ctx context.Context, config *Config) error {
	if config == nil || config.DomainID == "" {
		return fmt.Errorf("invalid config: missing required fields")
	}

	settings, _ := json.Marshal(config.Settings)

	_, err := r.db.ExecContext(ctx, `
		UPDATE idp_configurations SET provider_type = $1, config = $2, updated_at = now()
		WHERE domain_id = $3 AND status = 'active'
	`, config.ProviderType, settings, config.DomainID)

	return err
}

// DeleteConfig disables an IdP configuration (soft delete).
func (r *ConfigRepository) DeleteConfig(ctx context.Context, domainID string) error {
	if domainID == "" {
		return fmt.Errorf("invalid domain id")
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE idp_configurations SET status = 'disabled', updated_at = now()
		WHERE domain_id = $1
	`, domainID)

	return err
}
