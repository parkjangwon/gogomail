-- +goose Up
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_manifest_digests_algorithm_check'
  ) THEN
    ALTER TABLE api_usage_export_manifest_digests
      ADD CONSTRAINT api_usage_export_manifest_digests_algorithm_check
      CHECK (digest_algorithm = 'sha256');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_manifest_digests_hex_check'
  ) THEN
    ALTER TABLE api_usage_export_manifest_digests
      ADD CONSTRAINT api_usage_export_manifest_digests_hex_check
      CHECK (digest_hex ~ '^[0-9a-f]{64}$');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_manifest_digests_manifest_object_check'
  ) THEN
    ALTER TABLE api_usage_export_manifest_digests
      ADD CONSTRAINT api_usage_export_manifest_digests_manifest_object_check
      CHECK (jsonb_typeof(manifest) = 'object');
  END IF;
END $$;

-- +goose Down
