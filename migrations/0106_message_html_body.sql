-- +goose Up
ALTER TABLE messages
  ADD COLUMN IF NOT EXISTS html_body text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE messages
  DROP COLUMN IF EXISTS html_body;
