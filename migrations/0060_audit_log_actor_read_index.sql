-- +goose Up
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_time
  ON audit_logs (actor_id, created_at DESC, id DESC)
  WHERE actor_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_audit_logs_actor_time;
