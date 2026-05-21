-- +goose Up
CREATE INDEX IF NOT EXISTS idx_rdbms_sync_conflicts_domain_created_id
  ON rdbms_sync_conflicts(domain_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_rdbms_sync_conflicts_domain_unresolved_created_id
  ON rdbms_sync_conflicts(domain_id, created_at DESC, id DESC)
  WHERE resolution IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_rdbms_sync_conflicts_domain_unresolved_created_id;
DROP INDEX IF EXISTS idx_rdbms_sync_conflicts_domain_created_id;
