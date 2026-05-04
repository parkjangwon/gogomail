-- +goose Up
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS quota_source text NOT NULL DEFAULT 'default';

ALTER TABLE users
  ADD CONSTRAINT users_quota_source_check
  CHECK (quota_source IN ('default', 'custom'));

CREATE INDEX IF NOT EXISTS idx_users_domain_quota_source
  ON users(domain_id, quota_source);

-- +goose Down
DROP INDEX IF EXISTS idx_users_domain_quota_source;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_quota_source_check;
ALTER TABLE users DROP COLUMN IF EXISTS quota_source;
