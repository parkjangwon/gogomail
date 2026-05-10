-- +goose Up
ALTER TABLE api_usage_export_manifest_digests
  ADD CONSTRAINT api_usage_export_manifest_digests_algorithm_check
    CHECK (digest_algorithm = 'sha256');

ALTER TABLE api_usage_export_manifest_digests
  ADD CONSTRAINT api_usage_export_manifest_digests_hex_check
    CHECK (digest_hex ~ '^[0-9a-f]{64}$');

ALTER TABLE api_usage_export_manifest_digests
  ADD CONSTRAINT api_usage_export_manifest_digests_manifest_object_check
    CHECK (jsonb_typeof(manifest) = 'object');

-- +goose Down
ALTER TABLE api_usage_export_manifest_digests DROP CONSTRAINT IF EXISTS api_usage_export_manifest_digests_algorithm_check;
ALTER TABLE api_usage_export_manifest_digests DROP CONSTRAINT IF EXISTS api_usage_export_manifest_digests_hex_check;
ALTER TABLE api_usage_export_manifest_digests DROP CONSTRAINT IF EXISTS api_usage_export_manifest_digests_manifest_object_check;
