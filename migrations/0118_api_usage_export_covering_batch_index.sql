-- +goose Up
CREATE INDEX IF NOT EXISTS idx_api_usage_export_batches_covering_retention
  ON api_usage_export_batches (
    tenant_id,
    principal_id,
    COALESCE(window_start, '-infinity'::timestamptz),
    window_end,
    completed_at DESC,
    id DESC
  )
  WHERE status = 'completed'
    AND completed_at IS NOT NULL
    AND window_end IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_api_usage_export_batches_covering_retention;
