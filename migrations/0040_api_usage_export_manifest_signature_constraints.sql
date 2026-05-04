DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_manifest_signature_backend_check'
  ) THEN
    ALTER TABLE api_usage_export_manifest_signatures
      ADD CONSTRAINT api_usage_export_manifest_signature_backend_check
      CHECK (signer_backend <> '');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_manifest_signature_key_check'
  ) THEN
    ALTER TABLE api_usage_export_manifest_signatures
      ADD CONSTRAINT api_usage_export_manifest_signature_key_check
      CHECK (key_id <> '');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_manifest_signature_algorithm_check'
  ) THEN
    ALTER TABLE api_usage_export_manifest_signatures
      ADD CONSTRAINT api_usage_export_manifest_signature_algorithm_check
      CHECK (signature_algorithm = 'hmac-sha256');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_manifest_signature_metadata_check'
  ) THEN
    ALTER TABLE api_usage_export_manifest_signatures
      ADD CONSTRAINT api_usage_export_manifest_signature_metadata_check
      CHECK (jsonb_typeof(metadata) = 'object');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_manifest_signature_digest_check'
  ) THEN
    ALTER TABLE api_usage_export_manifest_signatures
      ADD CONSTRAINT api_usage_export_manifest_signature_digest_check
      CHECK (signed_digest_hex ~ '^[0-9a-f]{64}$');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_manifest_signature_hex_check'
  ) THEN
    ALTER TABLE api_usage_export_manifest_signatures
      ADD CONSTRAINT api_usage_export_manifest_signature_hex_check
      CHECK (signature_hex ~ '^[0-9a-f]{64}$');
  END IF;
END $$;
