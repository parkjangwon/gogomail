-- +goose Up
ALTER TABLE outbox
  ADD COLUMN locked_at timestamptz;

CREATE INDEX idx_outbox_retryable
  ON outbox(status, locked_at, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_outbox_retryable;

ALTER TABLE outbox
  DROP COLUMN IF EXISTS locked_at;
