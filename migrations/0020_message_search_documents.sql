-- +goose Up
CREATE TABLE message_search_documents (
  message_id uuid PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  body_text text NOT NULL DEFAULT '',
  body_text_truncated boolean NOT NULL DEFAULT false,
  indexed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_message_search_documents_user
  ON message_search_documents(user_id, indexed_at DESC);

CREATE INDEX idx_message_search_documents_body_fts
  ON message_search_documents
  USING gin(to_tsvector('simple', body_text));

-- +goose Down
DROP INDEX IF EXISTS idx_message_search_documents_body_fts;
DROP INDEX IF EXISTS idx_message_search_documents_user;
DROP TABLE IF EXISTS message_search_documents;
