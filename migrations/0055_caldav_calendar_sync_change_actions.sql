-- +goose Up
ALTER TABLE caldav_calendar_sync_changes
  DROP CONSTRAINT IF EXISTS caldav_calendar_sync_changes_action_check;

ALTER TABLE caldav_calendar_sync_changes
  ADD CONSTRAINT caldav_calendar_sync_changes_action_check
  CHECK (action IN ('collection-created', 'collection-updated', 'object-upserted', 'object-deleted', 'collection-deleted'));

-- +goose Down
DELETE FROM caldav_calendar_sync_changes
WHERE action = 'collection-updated';

ALTER TABLE caldav_calendar_sync_changes
  DROP CONSTRAINT IF EXISTS caldav_calendar_sync_changes_action_check;

ALTER TABLE caldav_calendar_sync_changes
  ADD CONSTRAINT caldav_calendar_sync_changes_action_check
  CHECK (action IN ('collection-created', 'object-upserted', 'object-deleted', 'collection-deleted'));
