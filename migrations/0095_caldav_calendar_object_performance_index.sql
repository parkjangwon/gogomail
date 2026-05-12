-- +goose Up
CREATE INDEX IF NOT EXISTS idx_caldav_calendar_objects_user_calendar_status_updated_id
  ON caldav_calendar_objects (user_id, calendar_id, status, updated_at DESC, id DESC)
  INCLUDE (object_name, uid, component_type, etag, size, created_at);

CREATE INDEX IF NOT EXISTS idx_caldav_calendar_objects_user_calendar_status_component_updated_id
  ON caldav_calendar_objects (user_id, calendar_id, status, component_type, updated_at DESC, id DESC)
  INCLUDE (object_name, uid, etag, size, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_caldav_calendar_objects_user_calendar_status_updated_id;
DROP INDEX IF EXISTS idx_caldav_calendar_objects_user_calendar_status_component_updated_id;
