-- +goose Up
CREATE INDEX IF NOT EXISTS idx_outbox_pending_available_claim
  ON outbox (available_at, created_at, id)
  WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_outbox_processing_locked_claim
  ON outbox (locked_at, created_at, id)
  WHERE status = 'processing';

-- +goose Down
DROP INDEX IF EXISTS idx_outbox_processing_locked_claim;
DROP INDEX IF EXISTS idx_outbox_pending_available_claim;
