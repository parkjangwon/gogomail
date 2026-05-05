-- +goose Up
CREATE TABLE IF NOT EXISTS caldav_calendars (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  normalized_name text NOT NULL,
  color text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  sync_token text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT caldav_calendars_name_check CHECK (name <> '' AND char_length(name) <= 255),
  CONSTRAINT caldav_calendars_normalized_name_check CHECK (normalized_name <> '' AND char_length(normalized_name) <= 255),
  CONSTRAINT caldav_calendars_color_check CHECK (color = '' OR color ~ '^#[0-9A-Fa-f]{6}$'),
  CONSTRAINT caldav_calendars_description_check CHECK (char_length(description) <= 2048),
  CONSTRAINT caldav_calendars_sync_token_check CHECK (sync_token <> '' AND char_length(sync_token) <= 128),
  CONSTRAINT caldav_calendars_status_check CHECK (status IN ('active', 'deleted'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_caldav_calendars_user_active_name
  ON caldav_calendars (user_id, normalized_name)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_caldav_calendars_user_status_updated
  ON caldav_calendars (user_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS caldav_calendar_objects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  calendar_id uuid NOT NULL REFERENCES caldav_calendars(id) ON DELETE CASCADE,
  object_name text NOT NULL,
  uid text NOT NULL,
  component_type text NOT NULL DEFAULT 'VEVENT',
  etag text NOT NULL,
  size bigint NOT NULL,
  ics text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT caldav_calendar_objects_name_check CHECK (object_name <> '' AND char_length(object_name) <= 200 AND lower(object_name) LIKE '%.ics'),
  CONSTRAINT caldav_calendar_objects_uid_check CHECK (uid <> '' AND char_length(uid) <= 255),
  CONSTRAINT caldav_calendar_objects_component_check CHECK (component_type IN ('VEVENT', 'VTODO', 'VJOURNAL', 'VFREEBUSY')),
  CONSTRAINT caldav_calendar_objects_etag_check CHECK (etag ~ '^"[0-9a-f]{64}"$'),
  CONSTRAINT caldav_calendar_objects_size_check CHECK (size >= 0 AND size <= 10485760),
  CONSTRAINT caldav_calendar_objects_ics_check CHECK (ics <> ''),
  CONSTRAINT caldav_calendar_objects_status_check CHECK (status IN ('active', 'deleted'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_caldav_calendar_objects_active_name
  ON caldav_calendar_objects (calendar_id, object_name)
  WHERE status = 'active';

CREATE UNIQUE INDEX IF NOT EXISTS idx_caldav_calendar_objects_active_uid
  ON caldav_calendar_objects (calendar_id, uid)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_caldav_calendar_objects_user_calendar_updated
  ON caldav_calendar_objects (user_id, calendar_id, status, updated_at DESC);

-- +goose Down
DROP TABLE IF EXISTS caldav_calendar_objects;
DROP TABLE IF EXISTS caldav_calendars;
