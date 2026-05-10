-- +goose Up
ALTER TABLE api_usage_export_manifest_signatures
  ADD CONSTRAINT api_usage_export_manifest_signature_backend_check
    CHECK (signer_backend <> '');

ALTER TABLE api_usage_export_manifest_signatures
  ADD CONSTRAINT api_usage_export_manifest_signature_key_check
    CHECK (key_id <> '');

ALTER TABLE api_usage_export_manifest_signatures
  ADD CONSTRAINT api_usage_export_manifest_signature_algorithm_check
    CHECK (signature_algorithm = 'hmac-sha256');

ALTER TABLE api_usage_export_manifest_signatures
  ADD CONSTRAINT api_usage_export_manifest_signature_metadata_check
    CHECK (jsonb_typeof(metadata) = 'object');

ALTER TABLE api_usage_export_manifest_signatures
  ADD CONSTRAINT api_usage_export_manifest_signature_digest_check
    CHECK (signed_digest_hex ~ '^[0-9a-f]{64}$');

ALTER TABLE api_usage_export_manifest_signatures
  ADD CONSTRAINT api_usage_export_manifest_signature_hex_check
    CHECK (signature_hex ~ '^[0-9a-f]{64}$');

-- +goose Down
ALTER TABLE api_usage_export_manifest_signatures DROP CONSTRAINT IF EXISTS api_usage_export_manifest_signature_backend_check;
ALTER TABLE api_usage_export_manifest_signatures DROP CONSTRAINT IF EXISTS api_usage_export_manifest_signature_key_check;
ALTER TABLE api_usage_export_manifest_signatures DROP CONSTRAINT IF EXISTS api_usage_export_manifest_signature_algorithm_check;
ALTER TABLE api_usage_export_manifest_signatures DROP CONSTRAINT IF EXISTS api_usage_export_manifest_signature_metadata_check;
ALTER TABLE api_usage_export_manifest_signatures DROP CONSTRAINT IF EXISTS api_usage_export_manifest_signature_digest_check;
ALTER TABLE api_usage_export_manifest_signatures DROP CONSTRAINT IF EXISTS api_usage_export_manifest_signature_hex_check;
