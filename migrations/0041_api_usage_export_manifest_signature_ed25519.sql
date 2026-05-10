-- +goose Up
ALTER TABLE api_usage_export_manifest_signatures
  DROP CONSTRAINT IF EXISTS api_usage_export_manifest_signature_algorithm_check;

ALTER TABLE api_usage_export_manifest_signatures
  ADD CONSTRAINT api_usage_export_manifest_signature_algorithm_check
    CHECK (signature_algorithm IN ('hmac-sha256', 'ed25519'));

ALTER TABLE api_usage_export_manifest_signatures
  DROP CONSTRAINT IF EXISTS api_usage_export_manifest_signature_backend_algorithm_check;

ALTER TABLE api_usage_export_manifest_signatures
  ADD CONSTRAINT api_usage_export_manifest_signature_backend_algorithm_check
    CHECK (
      (signer_backend = 'local-hmac' AND signature_algorithm = 'hmac-sha256')
      OR
      (signer_backend = 'local-ed25519' AND signature_algorithm = 'ed25519')
      OR
      signer_backend NOT IN ('local-hmac', 'local-ed25519')
    );

ALTER TABLE api_usage_export_manifest_signatures
  DROP CONSTRAINT IF EXISTS api_usage_export_manifest_signature_hex_check;

ALTER TABLE api_usage_export_manifest_signatures
  ADD CONSTRAINT api_usage_export_manifest_signature_hex_check
    CHECK (
      (signature_algorithm = 'hmac-sha256' AND signature_hex ~ '^[0-9a-f]{64}$')
      OR
      (signature_algorithm = 'ed25519' AND signature_hex ~ '^[0-9a-f]{128}$')
    );

-- +goose Down
ALTER TABLE api_usage_export_manifest_signatures
  DROP CONSTRAINT IF EXISTS api_usage_export_manifest_signature_algorithm_check,
  DROP CONSTRAINT IF EXISTS api_usage_export_manifest_signature_backend_algorithm_check,
  DROP CONSTRAINT IF EXISTS api_usage_export_manifest_signature_hex_check;

ALTER TABLE api_usage_export_manifest_signatures
  ADD CONSTRAINT api_usage_export_manifest_signature_algorithm_check
    CHECK (signature_algorithm = 'hmac-sha256');

ALTER TABLE api_usage_export_manifest_signatures
  ADD CONSTRAINT api_usage_export_manifest_signature_hex_check
    CHECK (signature_hex ~ '^[0-9a-f]{64}$');
