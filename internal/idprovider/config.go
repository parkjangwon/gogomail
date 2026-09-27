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

// SecretSettingKeys are the settings keys whose values are write-only secrets.
// They are never returned to clients in plaintext and must not be blanked by a
// no-change save.
var SecretSettingKeys = []string{"bind_password", "client_secret", "dsn"}

// SecretSetIndicator marks, in a redacted config, that a write-only secret is
// currently set on the server without disclosing its value. Clients render a
// "set" indicator when they observe this sentinel and only submit a new value
// when the operator explicitly types one.
const SecretSetIndicator = "__set__"

func isSecretKey(key string) bool {
	for _, k := range SecretSettingKeys {
		if k == key {
			return true
		}
	}
	return false
}

// RedactSecrets returns a shallow copy of cfg with every write-only secret in
// Settings replaced: a non-empty stored secret becomes SecretSetIndicator, an
// absent/empty one is removed. The original config is not mutated. Returns nil
// when cfg is nil.
func RedactSecrets(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}
	out := &Config{
		DomainID:     cfg.DomainID,
		ProviderType: cfg.ProviderType,
		Settings:     make(map[string]interface{}, len(cfg.Settings)),
	}
	for k, v := range cfg.Settings {
		if isSecretKey(k) {
			if s, ok := v.(string); ok && s != "" {
				out.Settings[k] = SecretSetIndicator
			}
			// empty/absent secret: omit entirely so clients show "not set".
			continue
		}
		out.Settings[k] = v
	}
	return out
}

// MergeSecrets reconciles secret settings on an incoming config against the
// currently stored config so that a save never silently blanks a secret. For
// each secret key, the incoming value is preserved only when the operator
// supplied a real new value; the SecretSetIndicator sentinel or an empty/absent
// value falls back to the stored secret. incoming is mutated in place. Both
// arguments may be nil (nil incoming is a no-op).
func MergeSecrets(incoming *Config, stored *Config) {
	if incoming == nil {
		return
	}
	if incoming.Settings == nil {
		incoming.Settings = map[string]interface{}{}
	}
	var storedSettings map[string]interface{}
	if stored != nil {
		storedSettings = stored.Settings
	}
	for _, key := range SecretSettingKeys {
		newVal, hasNew := incoming.Settings[key]
		newStr, _ := newVal.(string)
		// A real, non-empty, non-sentinel value from the operator wins.
		if hasNew && newStr != "" && newStr != SecretSetIndicator {
			continue
		}
		// Otherwise preserve the stored secret (if any).
		if storedSettings != nil {
			if prev, ok := storedSettings[key]; ok {
				if prevStr, ok := prev.(string); ok && prevStr != "" {
					incoming.Settings[key] = prevStr
					continue
				}
			}
		}
		// No stored secret to preserve: drop the sentinel/empty placeholder so
		// we never persist the indicator string as if it were a real secret.
		delete(incoming.Settings, key)
	}
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
