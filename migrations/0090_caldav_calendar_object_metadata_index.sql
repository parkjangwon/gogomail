-- +goose Up
CREATE INDEX IF NOT EXISTS idx_caldav_calendar_objects_active_metadata
  ON caldav_calendar_objects (user_id, calendar_id, status, updated_at DESC, id DESC)
  INCLUDE (object_name, uid, component_type, etag, size, created_at)
  WHERE status = 'active';

-- +goose Down
DROP INDEX IF EXISTS idx_caldav_calendar_objects_active_metadata;
