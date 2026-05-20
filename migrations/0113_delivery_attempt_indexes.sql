-- +goose Up
CREATE INDEX idx_delivery_attempts_status_attempted
  ON delivery_attempts (status, attempted_at DESC, id);

CREATE INDEX idx_delivery_attempts_recipient_domain_status_attempted
  ON delivery_attempts (recipient_domain, status, attempted_at DESC, id);

CREATE INDEX idx_delivery_attempts_message_status_attempted
  ON delivery_attempts (message_id, status, attempted_at DESC, id);

-- +goose Down
DROP INDEX IF EXISTS idx_delivery_attempts_status_attempted;
DROP INDEX IF EXISTS idx_delivery_attempts_recipient_domain_status_attempted;
DROP INDEX IF EXISTS idx_delivery_attempts_message_status_attempted;
