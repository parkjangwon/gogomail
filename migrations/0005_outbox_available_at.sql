-- +goose Up
ALTER TABLE outbox
  ADD COLUMN available_at timestamptz NOT NULL DEFAULT now();

CREATE INDEX idx_outbox_available
  ON outbox(status, available_at, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_outbox_available;

ALTER TABLE outbox
  DROP COLUMN IF EXISTS available_at;
