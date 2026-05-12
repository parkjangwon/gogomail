-- +goose Up
ALTER TABLE caldav_calendars
  ADD COLUMN displayname_lang text NOT NULL DEFAULT '',
  ADD COLUMN description_lang text NOT NULL DEFAULT '';

ALTER TABLE caldav_calendars
  ADD CONSTRAINT caldav_calendars_displayname_lang_check
    CHECK (char_length(displayname_lang) <= 64 AND displayname_lang !~ '[[:space:][:cntrl:]]'),
  ADD CONSTRAINT caldav_calendars_description_lang_check
    CHECK (char_length(description_lang) <= 64 AND description_lang !~ '[[:space:][:cntrl:]]');

ALTER TABLE carddav_addressbooks
  ADD COLUMN displayname_lang text NOT NULL DEFAULT '',
  ADD COLUMN description_lang text NOT NULL DEFAULT '';

ALTER TABLE carddav_addressbooks
  ADD CONSTRAINT carddav_addressbooks_displayname_lang_check
    CHECK (char_length(displayname_lang) <= 64 AND displayname_lang !~ '[[:space:][:cntrl:]]'),
  ADD CONSTRAINT carddav_addressbooks_description_lang_check
    CHECK (char_length(description_lang) <= 64 AND description_lang !~ '[[:space:][:cntrl:]]');

-- +goose Down
ALTER TABLE carddav_addressbooks
  DROP CONSTRAINT IF EXISTS carddav_addressbooks_description_lang_check,
  DROP CONSTRAINT IF EXISTS carddav_addressbooks_displayname_lang_check,
  DROP COLUMN IF EXISTS description_lang,
  DROP COLUMN IF EXISTS displayname_lang;

ALTER TABLE caldav_calendars
  DROP CONSTRAINT IF EXISTS caldav_calendars_description_lang_check,
  DROP CONSTRAINT IF EXISTS caldav_calendars_displayname_lang_check,
  DROP COLUMN IF EXISTS description_lang,
  DROP COLUMN IF EXISTS displayname_lang;
