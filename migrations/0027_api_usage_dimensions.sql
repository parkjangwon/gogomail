ALTER TABLE api_usage_events
  ADD COLUMN IF NOT EXISTS tenant_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS company_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS domain_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS api_key_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS principal_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS auth_source text NOT NULL DEFAULT '';

ALTER TABLE api_usage_daily
  ADD COLUMN IF NOT EXISTS tenant_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS company_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS domain_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS api_key_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS principal_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS auth_source text NOT NULL DEFAULT '';

ALTER TABLE api_usage_monthly
  ADD COLUMN IF NOT EXISTS tenant_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS company_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS domain_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS api_key_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS principal_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS auth_source text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_api_usage_events_principal_timestamp
  ON api_usage_events (principal_id, event_timestamp DESC)
  WHERE principal_id <> '';

CREATE INDEX IF NOT EXISTS idx_api_usage_events_tenant_timestamp
  ON api_usage_events (tenant_id, event_timestamp DESC)
  WHERE tenant_id <> '';
