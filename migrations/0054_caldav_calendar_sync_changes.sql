-- +goose Up
CREATE TABLE IF NOT EXISTS caldav_calendar_sync_changes (
  id bigserial PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  calendar_id uuid NOT NULL REFERENCES caldav_calendars(id) ON DELETE CASCADE,
  sync_token text NOT NULL,
  action text NOT NULL,
  object_name text NOT NULL DEFAULT '',
  etag text NOT NULL DEFAULT '',
  changed_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT caldav_calendar_sync_changes_token_check CHECK (sync_token <> '' AND char_length(sync_token) <= 128),
  CONSTRAINT caldav_calendar_sync_changes_action_check CHECK (action IN ('collection-created', 'object-upserted', 'object-deleted', 'collection-deleted')),
  CONSTRAINT caldav_calendar_sync_changes_object_name_check CHECK (char_length(object_name) <= 200),
  CONSTRAINT caldav_calendar_sync_changes_etag_check CHECK (etag = '' OR etag ~ '^"[0-9a-f]{64}"$')
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_caldav_calendar_sync_changes_calendar_token
  ON caldav_calendar_sync_changes (calendar_id, sync_token);

CREATE INDEX IF NOT EXISTS idx_caldav_calendar_sync_changes_calendar_id
  ON caldav_calendar_sync_changes (calendar_id, id);

-- +goose Down
DROP TABLE IF EXISTS caldav_calendar_sync_changes;
