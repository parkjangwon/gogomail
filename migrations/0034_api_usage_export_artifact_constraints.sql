-- +goose Up
ALTER TABLE api_usage_export_artifacts
  ADD CONSTRAINT api_usage_export_artifacts_content_type_check
    CHECK (content_type = 'application/x-ndjson');

ALTER TABLE api_usage_export_artifacts
  ADD CONSTRAINT api_usage_export_artifacts_counts_check
    CHECK (
      byte_count >= 0
      AND event_count >= 0
    );

ALTER TABLE api_usage_export_artifacts
  ADD CONSTRAINT api_usage_export_artifacts_sha256_check
    CHECK (sha256_hex ~ '^[0-9a-f]{64}$');

ALTER TABLE api_usage_export_artifacts
  ADD CONSTRAINT api_usage_export_artifacts_metadata_object_check
    CHECK (jsonb_typeof(metadata) = 'object');

-- +goose Down
ALTER TABLE api_usage_export_artifacts DROP CONSTRAINT IF EXISTS api_usage_export_artifacts_content_type_check;
ALTER TABLE api_usage_export_artifacts DROP CONSTRAINT IF EXISTS api_usage_export_artifacts_counts_check;
ALTER TABLE api_usage_export_artifacts DROP CONSTRAINT IF EXISTS api_usage_export_artifacts_sha256_check;
ALTER TABLE api_usage_export_artifacts DROP CONSTRAINT IF EXISTS api_usage_export_artifacts_metadata_object_check;
