-- +goose Up
CREATE TABLE IF NOT EXISTS api_usage_ledger (
  event_id text PRIMARY KEY,
  schema_version text NOT NULL,
  event_timestamp timestamptz NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT now(),
  method text NOT NULL,
  route text NOT NULL,
  status integer NOT NULL,
  tenant_id text NOT NULL DEFAULT '',
  company_id text NOT NULL DEFAULT '',
  domain_id text NOT NULL DEFAULT '',
  user_id text NOT NULL DEFAULT '',
  api_key_id text NOT NULL DEFAULT '',
  principal_id text NOT NULL DEFAULT '',
  auth_source text NOT NULL DEFAULT '',
  request_count bigint NOT NULL DEFAULT 1,
  request_bytes bigint NOT NULL DEFAULT 0,
  response_bytes bigint NOT NULL DEFAULT 0,
  latency_ms bigint NOT NULL DEFAULT 0,
  payload jsonb NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_api_usage_ledger_timestamp
  ON api_usage_ledger (event_timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_api_usage_ledger_tenant_timestamp
  ON api_usage_ledger (tenant_id, event_timestamp DESC)
  WHERE tenant_id <> '';

CREATE INDEX IF NOT EXISTS idx_api_usage_ledger_principal_timestamp
  ON api_usage_ledger (principal_id, event_timestamp DESC)
  WHERE principal_id <> '';

-- +goose Down
