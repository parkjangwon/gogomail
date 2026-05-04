-- +goose Up
CREATE TABLE IF NOT EXISTS api_usage_export_batches (
  id text PRIMARY KEY,
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  status text NOT NULL DEFAULT 'completed',
  export_format text NOT NULL DEFAULT 'ndjson',
  tenant_id text NOT NULL DEFAULT '',
  principal_id text NOT NULL DEFAULT '',
  window_start timestamptz,
  window_end timestamptz,
  event_count bigint NOT NULL DEFAULT 0,
  request_count bigint NOT NULL DEFAULT 0,
  request_bytes bigint NOT NULL DEFAULT 0,
  response_bytes bigint NOT NULL DEFAULT 0,
  latency_ms_total bigint NOT NULL DEFAULT 0,
  latency_ms_max bigint NOT NULL DEFAULT 0,
  first_event_at timestamptz,
  last_event_at timestamptz,
  manifest jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_api_usage_export_batches_created
  ON api_usage_export_batches (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_api_usage_export_batches_tenant_created
  ON api_usage_export_batches (tenant_id, created_at DESC)
  WHERE tenant_id <> '';

-- +goose Down
