-- +goose Up
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_batches_status_check'
  ) THEN
    ALTER TABLE api_usage_export_batches
      ADD CONSTRAINT api_usage_export_batches_status_check
      CHECK (status IN ('pending', 'completed', 'failed'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_batches_format_check'
  ) THEN
    ALTER TABLE api_usage_export_batches
      ADD CONSTRAINT api_usage_export_batches_format_check
      CHECK (export_format IN ('ndjson'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_batches_counts_check'
  ) THEN
    ALTER TABLE api_usage_export_batches
      ADD CONSTRAINT api_usage_export_batches_counts_check
      CHECK (
        event_count >= 0
        AND request_count >= 0
        AND request_bytes >= 0
        AND response_bytes >= 0
        AND latency_ms_total >= 0
        AND latency_ms_max >= 0
      );
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_batches_window_check'
  ) THEN
    ALTER TABLE api_usage_export_batches
      ADD CONSTRAINT api_usage_export_batches_window_check
      CHECK (
        window_start IS NULL
        OR window_end IS NULL
        OR window_start < window_end
      );
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_export_batches_manifest_object_check'
  ) THEN
    ALTER TABLE api_usage_export_batches
      ADD CONSTRAINT api_usage_export_batches_manifest_object_check
      CHECK (jsonb_typeof(manifest) = 'object');
  END IF;
END $$;

-- +goose Down
