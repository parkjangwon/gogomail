-- +goose Up
CREATE INDEX IF NOT EXISTS idx_rdbms_sync_runs_domain_created_id
  ON rdbms_sync_runs(domain_id, created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_rdbms_sync_runs_domain_created_id;
