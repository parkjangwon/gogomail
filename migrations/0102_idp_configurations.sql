-- +goose Up
CREATE TABLE IF NOT EXISTS idp_configurations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  provider_type text NOT NULL DEFAULT 'database',
  config jsonb NOT NULL DEFAULT '{}'::jsonb,
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT idp_configurations_provider_type_check CHECK (provider_type IN ('database', 'ldap', 'azure_ad', 'external_rdbms')),
  CONSTRAINT idp_configurations_status_check CHECK (status IN ('active', 'disabled'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_idp_configurations_domain_active
  ON idp_configurations (domain_id)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_idp_configurations_provider_type
  ON idp_configurations (provider_type);

-- +goose Down
DROP TABLE IF EXISTS idp_configurations;
