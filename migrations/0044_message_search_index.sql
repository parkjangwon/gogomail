-- +goose Up
CREATE INDEX IF NOT EXISTS idx_messages_search_simple
  ON messages USING GIN (
    to_tsvector(
      'simple',
      coalesce(subject, '') || ' ' ||
      coalesce(from_addr, '') || ' ' ||
      coalesce(from_name, '') || ' ' ||
      coalesce(draft_text_body, '')
    )
  )
  WHERE status = 'active';

-- +goose Down
DROP INDEX IF EXISTS idx_messages_search_simple;
