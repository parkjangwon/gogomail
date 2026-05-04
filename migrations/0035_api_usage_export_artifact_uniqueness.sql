-- +goose Up
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_usage_export_artifacts_batch_object_key
  ON api_usage_export_artifacts (batch_id, object_key);

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_usage_export_artifacts_batch_sha256
  ON api_usage_export_artifacts (batch_id, sha256_hex);

-- +goose Down
