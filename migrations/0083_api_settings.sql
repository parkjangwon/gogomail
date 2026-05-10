-- +goose Up
-- API Settings for rate limiting and CIDR allowlist

CREATE TABLE IF NOT EXISTS api_settings (
  domain_id uuid PRIMARY KEY REFERENCES domains(id) ON DELETE CASCADE,

  -- Rate Limiting
  rate_limit_rps int NOT NULL DEFAULT 100 CHECK (rate_limit_rps > 0),
  rate_limit_bps bigint NOT NULL DEFAULT 0 CHECK (rate_limit_bps >= 0),

  -- CIDR Allowlist
  cidr_allowlist_enabled boolean NOT NULL DEFAULT false,
  cidr_allowlist text[] DEFAULT '{}',

  -- API Key Requirement
  require_api_key boolean NOT NULL DEFAULT true,

  -- Audit
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL
);

-- Index for lookups
CREATE INDEX IF NOT EXISTS idx_api_settings_updated_at ON api_settings(updated_at DESC);

-- Trigger to auto-insert default settings when a new domain is created
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION create_default_api_settings()
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO api_settings (domain_id, updated_by)
  VALUES (NEW.id, NULL)
  ON CONFLICT (domain_id) DO NOTHING;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS after_domain_insert_api_settings ON domains;
CREATE TRIGGER after_domain_insert_api_settings
AFTER INSERT ON domains
FOR EACH ROW
EXECUTE FUNCTION create_default_api_settings();
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS after_domain_insert_api_settings ON domains;
DROP FUNCTION IF EXISTS create_default_api_settings();
DROP TABLE IF EXISTS api_settings;
