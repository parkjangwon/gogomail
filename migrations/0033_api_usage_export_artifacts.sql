CREATE TABLE IF NOT EXISTS api_usage_export_artifacts (
  id text PRIMARY KEY,
  batch_id text NOT NULL REFERENCES api_usage_export_batches(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  storage_backend text NOT NULL DEFAULT 'external',
  object_key text NOT NULL,
  content_type text NOT NULL DEFAULT 'application/x-ndjson',
  byte_count bigint NOT NULL,
  sha256_hex text NOT NULL,
  event_count bigint NOT NULL DEFAULT 0,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_api_usage_export_artifacts_batch_created
  ON api_usage_export_artifacts (batch_id, created_at DESC);
