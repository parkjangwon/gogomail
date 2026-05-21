-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_messages_active_to_addrs_trgm
  ON messages USING GIN ((to_addrs::text) gin_trgm_ops)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_messages_active_cc_addrs_trgm
  ON messages USING GIN ((cc_addrs::text) gin_trgm_ops)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_messages_active_bcc_addrs_trgm
  ON messages USING GIN ((bcc_addrs::text) gin_trgm_ops)
  WHERE status = 'active';

-- +goose Down
DROP INDEX IF EXISTS idx_messages_active_bcc_addrs_trgm;
DROP INDEX IF EXISTS idx_messages_active_cc_addrs_trgm;
DROP INDEX IF EXISTS idx_messages_active_to_addrs_trgm;
