-- +goose Up
CREATE INDEX IF NOT EXISTS idx_caldav_calendar_objects_active_lookup
  ON caldav_calendar_objects (user_id, calendar_id, object_name)
  WHERE status = 'active';

-- +goose Down
DROP INDEX IF EXISTS idx_caldav_calendar_objects_active_lookup;
