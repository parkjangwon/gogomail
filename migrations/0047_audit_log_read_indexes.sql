-- +goose Up
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at
  ON audit_logs (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_action_time
  ON audit_logs (category, action, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_target_time
  ON audit_logs (target_type, target_id, created_at DESC, id DESC)
  WHERE target_type <> '';

-- +goose Down
