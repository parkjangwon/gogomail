-- +goose Up
CREATE TABLE IF NOT EXISTS user_mcp_access_keys (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id          UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  domain_id        UUID        NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  key_hash         TEXT        NOT NULL UNIQUE,
  token_suffix     TEXT        NOT NULL,
  name             TEXT        NOT NULL,
  scopes           TEXT[]      NOT NULL DEFAULT '{}',
  permission_mode  TEXT        NOT NULL DEFAULT 'basic',
  allowed_cidrs    TEXT[]      NOT NULL DEFAULT '{}',
  expires_at       TIMESTAMPTZ,
  revoked          BOOLEAN     NOT NULL DEFAULT false,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_used_at     TIMESTAMPTZ,
  revoked_at       TIMESTAMPTZ,
  CONSTRAINT user_mcp_access_keys_name_nonempty CHECK (length(trim(name)) > 0),
  CONSTRAINT user_mcp_access_keys_permission_mode CHECK (permission_mode IN ('basic', 'bypass'))
);

CREATE INDEX IF NOT EXISTS idx_user_mcp_access_keys_user_active
  ON user_mcp_access_keys(user_id, created_at DESC)
  WHERE revoked = false;

CREATE INDEX IF NOT EXISTS idx_user_mcp_access_keys_hash
  ON user_mcp_access_keys(key_hash);

-- +goose Down
DROP INDEX IF EXISTS idx_user_mcp_access_keys_hash;
DROP INDEX IF EXISTS idx_user_mcp_access_keys_user_active;
DROP TABLE IF EXISTS user_mcp_access_keys;
