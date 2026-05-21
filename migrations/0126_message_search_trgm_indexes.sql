-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_messages_active_subject_trgm
  ON messages USING GIN (subject gin_trgm_ops)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_messages_active_from_addr_trgm
  ON messages USING GIN (from_addr gin_trgm_ops)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_messages_active_from_name_trgm
  ON messages USING GIN (from_name gin_trgm_ops)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_messages_active_metadata_fts
  ON messages USING GIN (
    to_tsvector(
      'simple',
      coalesce(subject, '') || ' ' ||
      coalesce(from_addr, '') || ' ' ||
      coalesce(from_name, '')
    )
  )
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_message_search_documents_body_trgm
  ON message_search_documents USING GIN (body_text gin_trgm_ops);

-- +goose Down
DROP INDEX IF EXISTS idx_message_search_documents_body_trgm;
DROP INDEX IF EXISTS idx_messages_active_metadata_fts;
DROP INDEX IF EXISTS idx_messages_active_from_name_trgm;
DROP INDEX IF EXISTS idx_messages_active_from_addr_trgm;
DROP INDEX IF EXISTS idx_messages_active_subject_trgm;
