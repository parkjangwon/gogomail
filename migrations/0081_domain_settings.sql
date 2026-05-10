-- Domain Settings for TLS policy, quota, IP whitelist, 2FA, session timeout, password policy

CREATE TABLE IF NOT EXISTS domain_settings (
  domain_id TEXT PRIMARY KEY REFERENCES companies(domain) ON DELETE CASCADE,

  -- TLS Policy
  tls_policy TEXT NOT NULL DEFAULT 'opportunistic'
    CHECK (tls_policy IN ('opportunistic', 'require', 'disable')),

  -- Storage Quota per User (in bytes)
  quota_per_user BIGINT NOT NULL DEFAULT 10737418240, -- 10GB default

  -- IP Whitelist
  ip_whitelist_enabled BOOLEAN NOT NULL DEFAULT false,
  ip_whitelist TEXT[] DEFAULT '{}',

  -- 2FA Requirement
  require_2fa BOOLEAN NOT NULL DEFAULT false,

  -- Session Management
  session_timeout_minutes INT NOT NULL DEFAULT 480 CHECK (session_timeout_minutes > 0),

  -- Password Policy
  password_min_length INT NOT NULL DEFAULT 8 CHECK (password_min_length > 0),
  password_require_uppercase BOOLEAN NOT NULL DEFAULT true,
  password_require_numbers BOOLEAN NOT NULL DEFAULT true,
  password_require_special_chars BOOLEAN NOT NULL DEFAULT false,
  password_expiry_days INT NOT NULL DEFAULT 0 CHECK (password_expiry_days >= 0),

  -- Audit
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by TEXT NOT NULL REFERENCES admin_users(id),

  CONSTRAINT valid_quota CHECK (quota_per_user > 0)
);

-- Index for lookups
CREATE INDEX IF NOT EXISTS idx_domain_settings_updated_at ON domain_settings(updated_at DESC);

-- Trigger to auto-insert default settings when a new company is created
CREATE OR REPLACE FUNCTION create_default_domain_settings()
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO domain_settings (domain_id, updated_by)
  VALUES (NEW.domain, NEW.created_by)
  ON CONFLICT (domain_id) DO NOTHING;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS after_company_insert_settings ON companies;
CREATE TRIGGER after_company_insert_settings
AFTER INSERT ON companies
FOR EACH ROW
EXECUTE FUNCTION create_default_domain_settings();

COMMIT;
