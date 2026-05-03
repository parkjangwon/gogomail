-- +goose Up
ALTER TABLE outbox
  ADD COLUMN dedupe_key text;

CREATE UNIQUE INDEX idx_outbox_dedupe_key
  ON outbox(dedupe_key)
  WHERE dedupe_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_outbox_dedupe_key;

ALTER TABLE outbox
  DROP COLUMN IF EXISTS dedupe_key;
