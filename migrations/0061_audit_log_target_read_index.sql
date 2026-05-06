-- +goose Up
CREATE INDEX IF NOT EXISTS idx_audit_logs_target_id_time
  ON audit_logs (target_id, created_at DESC, id DESC)
  WHERE target_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_audit_logs_target_id_time;
