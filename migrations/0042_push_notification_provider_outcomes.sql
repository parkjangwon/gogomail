-- +goose Up
ALTER TABLE push_notification_attempts
  ADD COLUMN IF NOT EXISTS provider_message_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS provider_status text NOT NULL DEFAULT '';

ALTER TABLE push_notification_attempts
  ADD CONSTRAINT push_notification_attempts_provider_message_id_length_check
    CHECK (char_length(provider_message_id) <= 500);

ALTER TABLE push_notification_attempts
  ADD CONSTRAINT push_notification_attempts_provider_status_length_check
    CHECK (char_length(provider_status) <= 500);

-- +goose Down
ALTER TABLE push_notification_attempts
  DROP CONSTRAINT IF EXISTS push_notification_attempts_provider_message_id_length_check,
  DROP CONSTRAINT IF EXISTS push_notification_attempts_provider_status_length_check,
  DROP COLUMN IF EXISTS provider_message_id,
  DROP COLUMN IF EXISTS provider_status;
