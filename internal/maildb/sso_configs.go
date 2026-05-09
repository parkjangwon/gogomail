package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type SSOConfig struct {
	DomainID        string    `json:"domain_id"`
	Provider        string    `json:"provider"` // "saml" or "oidc"
	EntityID        string    `json:"entity_id,omitempty"`
	SSOURL          string    `json:"sso_url,omitempty"`
	Certificate     string    `json:"certificate,omitempty"`
	ClientID        string    `json:"client_id,omitempty"`
	ClientSecret    string    `json:"client_secret,omitempty"`
	DiscoveryURL    string    `json:"discovery_url,omitempty"`
	ACSURL          string    `json:"acs_url,omitempty"`
	JITProvisioning    bool      `json:"jit_provisioning"`
	SessionTTLSeconds  int       `json:"session_ttl_seconds"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func ValidateSSOConfig(cfg SSOConfig) error {
	if cfg.DomainID == "" {
		return fmt.Errorf("domain_id is required")
	}
	if cfg.Provider != "saml" && cfg.Provider != "oidc" {
		return fmt.Errorf("provider must be saml or oidc")
	}
	if cfg.Provider == "saml" && cfg.SSOURL == "" {
		return fmt.Errorf("sso_url is required for SAML provider")
	}
	if cfg.Provider == "oidc" && cfg.ClientID == "" {
		return fmt.Errorf("client_id is required for OIDC provider")
	}
	return nil
}

func (r *Repository) GetSSOConfig(ctx context.Context, domainID string) (SSOConfig, error) {
	if r.db == nil {
		return SSOConfig{}, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT domain_id::text, provider,
       COALESCE(entity_id, ''), COALESCE(sso_url, ''),
       COALESCE(certificate, ''), COALESCE(client_id, ''),
       COALESCE(client_secret, ''), COALESCE(discovery_url, ''),
       COALESCE(acs_url, ''), jit_provisioning, session_ttl_seconds,
       created_at, updated_at
FROM sso_configurations
WHERE domain_id = $1::uuid`
	var cfg SSOConfig
	err := r.db.QueryRowContext(ctx, query, domainID).Scan(
		&cfg.DomainID, &cfg.Provider,
		&cfg.EntityID, &cfg.SSOURL,
		&cfg.Certificate, &cfg.ClientID,
		&cfg.ClientSecret, &cfg.DiscoveryURL,
		&cfg.ACSURL, &cfg.JITProvisioning, &cfg.SessionTTLSeconds,
		&cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return SSOConfig{}, fmt.Errorf("sso configuration not found")
	}
	return cfg, err
}

func (r *Repository) UpsertSSOConfig(ctx context.Context, cfg SSOConfig) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateSSOConfig(cfg); err != nil {
		return err
	}
	const query = `
INSERT INTO sso_configurations
  (domain_id, provider, entity_id, sso_url, certificate,
   client_id, client_secret, discovery_url, acs_url, jit_provisioning,
   session_ttl_seconds)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (domain_id) DO UPDATE SET
  provider            = EXCLUDED.provider,
  entity_id           = EXCLUDED.entity_id,
  sso_url             = EXCLUDED.sso_url,
  certificate         = EXCLUDED.certificate,
  client_id           = EXCLUDED.client_id,
  client_secret       = EXCLUDED.client_secret,
  discovery_url       = EXCLUDED.discovery_url,
  acs_url             = EXCLUDED.acs_url,
  jit_provisioning    = EXCLUDED.jit_provisioning,
  session_ttl_seconds = EXCLUDED.session_ttl_seconds,
  updated_at          = NOW()`
	_, err := r.db.ExecContext(ctx, query,
		cfg.DomainID, cfg.Provider,
		cfg.EntityID, cfg.SSOURL,
		cfg.Certificate, cfg.ClientID,
		cfg.ClientSecret, cfg.DiscoveryURL,
		cfg.ACSURL, cfg.JITProvisioning,
		cfg.SessionTTLSeconds,
	)
	return err
}

func (r *Repository) DeleteSSOConfig(ctx context.Context, domainID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	const query = `DELETE FROM sso_configurations WHERE domain_id = $1::uuid`
	result, err := r.db.ExecContext(ctx, query, domainID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("sso configuration not found")
	}
	return nil
}
