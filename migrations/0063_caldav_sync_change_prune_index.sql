-- +goose Up
CREATE INDEX IF NOT EXISTS idx_caldav_calendar_sync_changes_prune
  ON caldav_calendar_sync_changes (changed_at, id);

-- +goose Down
DROP INDEX IF EXISTS idx_caldav_calendar_sync_changes_prune;
