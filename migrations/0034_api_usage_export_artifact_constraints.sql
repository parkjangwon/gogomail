-- +goose Up
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_artifacts_content_type_check'
  ) THEN
    ALTER TABLE api_usage_export_artifacts
      ADD CONSTRAINT api_usage_export_artifacts_content_type_check
      CHECK (content_type = 'application/x-ndjson');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_artifacts_counts_check'
  ) THEN
    ALTER TABLE api_usage_export_artifacts
      ADD CONSTRAINT api_usage_export_artifacts_counts_check
      CHECK (
        byte_count >= 0
        AND event_count >= 0
      );
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_artifacts_sha256_check'
  ) THEN
    ALTER TABLE api_usage_export_artifacts
      ADD CONSTRAINT api_usage_export_artifacts_sha256_check
      CHECK (sha256_hex ~ '^[0-9a-f]{64}$');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_artifacts_metadata_object_check'
  ) THEN
    ALTER TABLE api_usage_export_artifacts
      ADD CONSTRAINT api_usage_export_artifacts_metadata_object_check
      CHECK (jsonb_typeof(metadata) = 'object');
  END IF;
END $$;

-- +goose Down
