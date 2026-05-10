-- +goose Up
-- API Keys for domain access control

CREATE TABLE IF NOT EXISTS api_keys (
  id uuid PRIMARY KEY,
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,

  -- Key metadata
  name text NOT NULL,
  secret_hash text NOT NULL UNIQUE,

  -- Created by and timestamps
  created_by uuid NOT NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_used_at timestamptz,
  expires_at timestamptz,

  -- Status
  is_active boolean NOT NULL DEFAULT true,

  -- Constraints
  CONSTRAINT valid_name CHECK (length(name) > 0),
  CONSTRAINT valid_secret_hash CHECK (length(secret_hash) > 0)
);

-- Indexes for lookups
CREATE INDEX IF NOT EXISTS idx_api_keys_domain_id ON api_keys(domain_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_is_active ON api_keys(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_api_keys_created_at ON api_keys(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_api_keys_last_used_at ON api_keys(last_used_at DESC) WHERE last_used_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_api_keys_last_used_at;
DROP INDEX IF EXISTS idx_api_keys_created_at;
DROP INDEX IF EXISTS idx_api_keys_is_active;
DROP INDEX IF EXISTS idx_api_keys_domain_id;
DROP TABLE IF EXISTS api_keys;
