DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_manifest_signature_algorithm_check'
  ) THEN
    ALTER TABLE api_usage_export_manifest_signatures
      DROP CONSTRAINT api_usage_export_manifest_signature_algorithm_check;
  END IF;

  ALTER TABLE api_usage_export_manifest_signatures
    ADD CONSTRAINT api_usage_export_manifest_signature_algorithm_check
    CHECK (signature_algorithm IN ('hmac-sha256', 'ed25519'));
END $$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_manifest_signature_backend_algorithm_check'
  ) THEN
    ALTER TABLE api_usage_export_manifest_signatures
      DROP CONSTRAINT api_usage_export_manifest_signature_backend_algorithm_check;
  END IF;

  ALTER TABLE api_usage_export_manifest_signatures
    ADD CONSTRAINT api_usage_export_manifest_signature_backend_algorithm_check
    CHECK (
      (signer_backend = 'local-hmac' AND signature_algorithm = 'hmac-sha256')
      OR
      (signer_backend = 'local-ed25519' AND signature_algorithm = 'ed25519')
      OR
      signer_backend NOT IN ('local-hmac', 'local-ed25519')
    );
END $$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_manifest_signature_hex_check'
  ) THEN
    ALTER TABLE api_usage_export_manifest_signatures
      DROP CONSTRAINT api_usage_export_manifest_signature_hex_check;
  END IF;

  ALTER TABLE api_usage_export_manifest_signatures
    ADD CONSTRAINT api_usage_export_manifest_signature_hex_check
    CHECK (
      (signature_algorithm = 'hmac-sha256' AND signature_hex ~ '^[0-9a-f]{64}$')
      OR
      (signature_algorithm = 'ed25519' AND signature_hex ~ '^[0-9a-f]{128}$')
    );
END $$;
