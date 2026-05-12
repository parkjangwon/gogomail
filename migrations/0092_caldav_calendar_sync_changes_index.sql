-- +goose Up
CREATE INDEX IF NOT EXISTS idx_caldav_calendar_sync_changes_user_calendar_token
  ON caldav_calendar_sync_changes (user_id, calendar_id, sync_token);

CREATE INDEX IF NOT EXISTS idx_caldav_calendar_sync_changes_user_calendar_id
  ON caldav_calendar_sync_changes (user_id, calendar_id, id);

-- +goose Down
DROP INDEX IF EXISTS idx_caldav_calendar_sync_changes_user_calendar_id;
DROP INDEX IF EXISTS idx_caldav_calendar_sync_changes_user_calendar_token;
