-- +goose Up
-- Domain Settings for TLS policy, quota, IP whitelist, 2FA, session timeout, password policy

CREATE TABLE IF NOT EXISTS domain_settings (
  domain_id uuid PRIMARY KEY REFERENCES domains(id) ON DELETE CASCADE,

  -- TLS Policy
  tls_policy text NOT NULL DEFAULT 'opportunistic'
    CHECK (tls_policy IN ('opportunistic', 'require', 'disable')),

  -- Storage Quota per User (in bytes)
  quota_per_user bigint NOT NULL DEFAULT 10737418240, -- 10GB default

  -- IP Whitelist
  ip_whitelist_enabled boolean NOT NULL DEFAULT false,
  ip_whitelist text[] DEFAULT '{}',

  -- 2FA Requirement
  require_2fa boolean NOT NULL DEFAULT false,

  -- Session Management
  session_timeout_minutes int NOT NULL DEFAULT 480 CHECK (session_timeout_minutes > 0),

  -- Password Policy
  password_min_length int NOT NULL DEFAULT 8 CHECK (password_min_length > 0),
  password_require_uppercase boolean NOT NULL DEFAULT true,
  password_require_numbers boolean NOT NULL DEFAULT true,
  password_require_special_chars boolean NOT NULL DEFAULT false,
  password_expiry_days int NOT NULL DEFAULT 0 CHECK (password_expiry_days >= 0),
  password_reset_token_ttl_minutes int NOT NULL DEFAULT 60 CHECK (password_reset_token_ttl_minutes > 0 AND password_reset_token_ttl_minutes <= 10080),

  -- Audit
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,

  CONSTRAINT valid_quota CHECK (quota_per_user > 0)
);

-- Index for lookups
CREATE INDEX IF NOT EXISTS idx_domain_settings_updated_at ON domain_settings(updated_at DESC);

-- Trigger to auto-insert default settings when a new domain is created
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION create_default_domain_settings()
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO domain_settings (domain_id, updated_by)
  VALUES (NEW.id, NULL)
  ON CONFLICT (domain_id) DO NOTHING;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS after_domain_insert_settings ON domains;
CREATE TRIGGER after_domain_insert_settings
AFTER INSERT ON domains
FOR EACH ROW
EXECUTE FUNCTION create_default_domain_settings();
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS after_company_insert_settings ON companies;
DROP FUNCTION IF EXISTS create_default_domain_settings();
DROP TABLE IF EXISTS domain_settings;
