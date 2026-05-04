ALTER TABLE push_notification_attempts
  ADD COLUMN IF NOT EXISTS provider_message_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS provider_status text NOT NULL DEFAULT '';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'push_notification_attempts_provider_message_id_length_check'
  ) THEN
    ALTER TABLE push_notification_attempts
      ADD CONSTRAINT push_notification_attempts_provider_message_id_length_check
      CHECK (char_length(provider_message_id) <= 500);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'push_notification_attempts_provider_status_length_check'
  ) THEN
    ALTER TABLE push_notification_attempts
      ADD CONSTRAINT push_notification_attempts_provider_status_length_check
      CHECK (char_length(provider_status) <= 500);
  END IF;
END $$;
