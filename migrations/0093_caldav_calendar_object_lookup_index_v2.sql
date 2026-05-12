-- +goose Up
CREATE INDEX IF NOT EXISTS idx_caldav_calendar_objects_user_status_calendar_name
  ON caldav_calendar_objects (user_id, status, calendar_id, object_name);

-- +goose Down
DROP INDEX IF EXISTS idx_caldav_calendar_objects_user_status_calendar_name;
