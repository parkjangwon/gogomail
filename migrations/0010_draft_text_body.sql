-- +goose Up
ALTER TABLE messages
  ADD COLUMN draft_text_body text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE messages
  DROP COLUMN IF EXISTS draft_text_body;
