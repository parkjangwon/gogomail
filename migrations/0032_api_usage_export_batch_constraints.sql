-- +goose Up
ALTER TABLE api_usage_export_batches
  ADD CONSTRAINT api_usage_export_batches_status_check
    CHECK (status IN ('pending', 'completed', 'failed'));

ALTER TABLE api_usage_export_batches
  ADD CONSTRAINT api_usage_export_batches_format_check
    CHECK (export_format IN ('ndjson'));

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

ALTER TABLE api_usage_export_batches
  ADD CONSTRAINT api_usage_export_batches_window_check
    CHECK (
      window_start IS NULL
      OR window_end IS NULL
      OR window_start < window_end
    );

ALTER TABLE api_usage_export_batches
  ADD CONSTRAINT api_usage_export_batches_manifest_object_check
    CHECK (jsonb_typeof(manifest) = 'object');

-- +goose Down
ALTER TABLE api_usage_export_batches DROP CONSTRAINT IF EXISTS api_usage_export_batches_status_check;
ALTER TABLE api_usage_export_batches DROP CONSTRAINT IF EXISTS api_usage_export_batches_format_check;
ALTER TABLE api_usage_export_batches DROP CONSTRAINT IF EXISTS api_usage_export_batches_counts_check;
ALTER TABLE api_usage_export_batches DROP CONSTRAINT IF EXISTS api_usage_export_batches_window_check;
ALTER TABLE api_usage_export_batches DROP CONSTRAINT IF EXISTS api_usage_export_batches_manifest_object_check;
