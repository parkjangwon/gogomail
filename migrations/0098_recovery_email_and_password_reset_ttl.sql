-- +goose Up
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS recovery_email text NOT NULL DEFAULT '',
  ADD CONSTRAINT users_recovery_email_format
    CHECK (
      recovery_email = ''
      OR (
        char_length(recovery_email) <= 320
        AND recovery_email !~ '[[:space:][:cntrl:]]'
        AND recovery_email ~* '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'
      )
    );

ALTER TABLE domain_settings
  ADD COLUMN IF NOT EXISTS password_reset_token_ttl_minutes int NOT NULL DEFAULT 60;

-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'domain_settings_password_reset_token_ttl_minutes_check'
  ) THEN
    ALTER TABLE domain_settings
      ADD CONSTRAINT domain_settings_password_reset_token_ttl_minutes_check
      CHECK (password_reset_token_ttl_minutes > 0 AND password_reset_token_ttl_minutes <= 10080);
  END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE domain_settings
  DROP CONSTRAINT IF EXISTS domain_settings_password_reset_token_ttl_minutes_check,
  DROP COLUMN IF EXISTS password_reset_token_ttl_minutes;

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS users_recovery_email_format,
  DROP COLUMN IF EXISTS recovery_email;
