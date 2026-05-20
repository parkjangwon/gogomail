-- +goose Up
ALTER TABLE quota_alerts
  ADD COLUMN IF NOT EXISTS notified_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_quota_alerts_user_unnotified
  ON quota_alerts (created_at ASC, id ASC)
  WHERE scope = 'user' AND notified_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_quota_alerts_user_unnotified;
ALTER TABLE quota_alerts
  DROP COLUMN IF EXISTS notified_at;
