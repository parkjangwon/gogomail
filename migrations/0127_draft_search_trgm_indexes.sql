-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_messages_draft_subject_trgm
  ON messages USING GIN (subject gin_trgm_ops)
  WHERE status = 'draft';

CREATE INDEX IF NOT EXISTS idx_messages_draft_from_addr_trgm
  ON messages USING GIN (from_addr gin_trgm_ops)
  WHERE status = 'draft';

CREATE INDEX IF NOT EXISTS idx_messages_draft_from_name_trgm
  ON messages USING GIN (from_name gin_trgm_ops)
  WHERE status = 'draft';

CREATE INDEX IF NOT EXISTS idx_messages_draft_to_addrs_trgm
  ON messages USING GIN ((to_addrs::text) gin_trgm_ops)
  WHERE status = 'draft';

CREATE INDEX IF NOT EXISTS idx_messages_draft_cc_addrs_trgm
  ON messages USING GIN ((cc_addrs::text) gin_trgm_ops)
  WHERE status = 'draft';

CREATE INDEX IF NOT EXISTS idx_messages_draft_bcc_addrs_trgm
  ON messages USING GIN ((bcc_addrs::text) gin_trgm_ops)
  WHERE status = 'draft';

CREATE INDEX IF NOT EXISTS idx_messages_draft_text_body_trgm
  ON messages USING GIN (draft_text_body gin_trgm_ops)
  WHERE status = 'draft';

-- +goose Down
DROP INDEX IF EXISTS idx_messages_draft_text_body_trgm;
DROP INDEX IF EXISTS idx_messages_draft_bcc_addrs_trgm;
DROP INDEX IF EXISTS idx_messages_draft_cc_addrs_trgm;
DROP INDEX IF EXISTS idx_messages_draft_to_addrs_trgm;
DROP INDEX IF EXISTS idx_messages_draft_from_name_trgm;
DROP INDEX IF EXISTS idx_messages_draft_from_addr_trgm;
DROP INDEX IF EXISTS idx_messages_draft_subject_trgm;
