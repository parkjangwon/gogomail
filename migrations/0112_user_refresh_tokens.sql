-- +goose Up
CREATE TABLE IF NOT EXISTS user_refresh_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_refresh_tokens_user_active
  ON user_refresh_tokens (user_id, expires_at DESC)
  WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS user_refresh_tokens;
