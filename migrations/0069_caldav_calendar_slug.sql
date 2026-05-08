-- +goose Up
ALTER TABLE caldav_calendars ADD COLUMN slug text;
CREATE UNIQUE INDEX IF NOT EXISTS idx_caldav_calendars_user_active_slug
  ON caldav_calendars (user_id, lower(slug))
  WHERE status = 'active' AND slug IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_caldav_calendars_user_active_slug;
ALTER TABLE caldav_calendars DROP COLUMN IF EXISTS slug;