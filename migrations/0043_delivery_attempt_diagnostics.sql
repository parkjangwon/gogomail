-- +goose Up
ALTER TABLE delivery_attempts
  ADD COLUMN IF NOT EXISTS sender text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS enhanced_status text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS dsn_return text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS dsn_envelope_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS dsn_notify jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS original_recipient text NOT NULL DEFAULT '';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'delivery_attempts_sender_length_check'
  ) THEN
    ALTER TABLE delivery_attempts
      ADD CONSTRAINT delivery_attempts_sender_length_check
      CHECK (char_length(sender) <= 320);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'delivery_attempts_enhanced_status_length_check'
  ) THEN
    ALTER TABLE delivery_attempts
      ADD CONSTRAINT delivery_attempts_enhanced_status_length_check
      CHECK (char_length(enhanced_status) <= 64);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'delivery_attempts_dsn_return_length_check'
  ) THEN
    ALTER TABLE delivery_attempts
      ADD CONSTRAINT delivery_attempts_dsn_return_length_check
      CHECK (char_length(dsn_return) <= 16);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'delivery_attempts_dsn_envelope_id_length_check'
  ) THEN
    ALTER TABLE delivery_attempts
      ADD CONSTRAINT delivery_attempts_dsn_envelope_id_length_check
      CHECK (char_length(dsn_envelope_id) <= 100);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'delivery_attempts_original_recipient_length_check'
  ) THEN
    ALTER TABLE delivery_attempts
      ADD CONSTRAINT delivery_attempts_original_recipient_length_check
      CHECK (char_length(original_recipient) <= 500);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'delivery_attempts_dsn_notify_array_check'
  ) THEN
    ALTER TABLE delivery_attempts
      ADD CONSTRAINT delivery_attempts_dsn_notify_array_check
      CHECK (jsonb_typeof(dsn_notify) = 'array');
  END IF;
END $$;

-- +goose Down
ALTER TABLE delivery_attempts
  DROP CONSTRAINT IF EXISTS delivery_attempts_sender_length_check,
  DROP CONSTRAINT IF EXISTS delivery_attempts_enhanced_status_length_check,
  DROP CONSTRAINT IF EXISTS delivery_attempts_dsn_return_length_check,
  DROP CONSTRAINT IF EXISTS delivery_attempts_dsn_envelope_id_length_check,
  DROP CONSTRAINT IF EXISTS delivery_attempts_original_recipient_length_check,
  DROP CONSTRAINT IF EXISTS delivery_attempts_dsn_notify_array_check,
  DROP COLUMN IF EXISTS sender,
  DROP COLUMN IF EXISTS enhanced_status,
  DROP COLUMN IF EXISTS dsn_return,
  DROP COLUMN IF EXISTS dsn_envelope_id,
  DROP COLUMN IF EXISTS dsn_notify,
  DROP COLUMN IF EXISTS original_recipient;
