-- +goose Up
CREATE INDEX IF NOT EXISTS idx_caldav_calendar_sync_changes_user_calendar_id_covering
  ON caldav_calendar_sync_changes (user_id, calendar_id, id)
  INCLUDE (object_name, etag, action, sync_token);

-- +goose Down
DROP INDEX IF EXISTS idx_caldav_calendar_sync_changes_user_calendar_id_covering;
