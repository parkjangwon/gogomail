-- +goose Up
ALTER TABLE audit_logs
  ADD COLUMN hash text NOT NULL DEFAULT '';

CREATE INDEX idx_audit_logs_hash
  ON audit_logs(hash);

-- +goose Down
DROP INDEX IF EXISTS idx_audit_logs_hash;

ALTER TABLE audit_logs
  DROP COLUMN IF EXISTS hash;
