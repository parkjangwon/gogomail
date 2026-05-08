-- +goose Up
ALTER TABLE caldav_calendars ADD COLUMN timezone text;

-- +goose Down
ALTER TABLE caldav_calendars DROP COLUMN IF EXISTS timezone;