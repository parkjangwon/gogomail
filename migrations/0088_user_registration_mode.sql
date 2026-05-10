-- +goose Up
-- User registration mode: temp_password or email_invite, per domain

ALTER TABLE domain_settings
  ADD COLUMN user_registration_mode text NOT NULL DEFAULT 'temp_password'
    CHECK (user_registration_mode IN ('temp_password', 'email_invite'));

ALTER TABLE users
  ADD COLUMN must_change_password boolean NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS user_invite_tokens (
  id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid        NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
  domain_id  uuid        NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  token      text        NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  accepted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by uuid        REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_user_invite_tokens_token   ON user_invite_tokens(token);
CREATE INDEX IF NOT EXISTS idx_user_invite_tokens_user_id ON user_invite_tokens(user_id);

-- +goose Down
DROP TABLE IF EXISTS user_invite_tokens;
ALTER TABLE users      DROP COLUMN IF EXISTS must_change_password;
ALTER TABLE domain_settings DROP COLUMN IF EXISTS user_registration_mode;
