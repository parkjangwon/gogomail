CREATE TABLE IF NOT EXISTS api_usage_export_manifest_signatures (
  id text PRIMARY KEY,
  digest_id text NOT NULL REFERENCES api_usage_export_manifest_digests(id) ON DELETE CASCADE,
  batch_id text NOT NULL REFERENCES api_usage_export_batches(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  signer_backend text NOT NULL,
  key_id text NOT NULL,
  signature_algorithm text NOT NULL,
  signed_digest_hex text NOT NULL,
  signature_hex text NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_usage_export_manifest_signatures_digest_key
  ON api_usage_export_manifest_signatures (digest_id, signature_algorithm, key_id, signature_hex);

CREATE INDEX IF NOT EXISTS idx_api_usage_export_manifest_signatures_batch_created
  ON api_usage_export_manifest_signatures (batch_id, created_at DESC);
