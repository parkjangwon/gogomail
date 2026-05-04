CREATE TABLE IF NOT EXISTS api_usage_export_manifest_digests (
  id text PRIMARY KEY,
  batch_id text NOT NULL REFERENCES api_usage_export_batches(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  schema_version text NOT NULL,
  digest_algorithm text NOT NULL DEFAULT 'sha256',
  digest_hex text NOT NULL,
  manifest jsonb NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_usage_export_manifest_digests_batch_digest
  ON api_usage_export_manifest_digests (batch_id, digest_algorithm, digest_hex);

CREATE INDEX IF NOT EXISTS idx_api_usage_export_manifest_digests_batch_created
  ON api_usage_export_manifest_digests (batch_id, created_at DESC);
