-- +goose Up
-- Retention/cleanup queries on config_change_log scan by scope + recency.
-- Add a composite index ordered DESC on created_at to support both
-- "latest changes for scope" lookups and time-bound retention deletes.
CREATE INDEX IF NOT EXISTS idx_config_change_log_scope_created
    ON config_change_log(scope_type, scope_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_config_change_log_scope_created;
