-- +goose Up
CREATE TABLE delivery_attempts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  message_id uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  rfc_message_id text NOT NULL DEFAULT '',
  farm text NOT NULL,
  recipient text NOT NULL,
  recipient_domain text NOT NULL DEFAULT '',
  status text NOT NULL,
  error_message text NOT NULL DEFAULT '',
  attempted_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_delivery_attempts_message_time
  ON delivery_attempts(message_id, attempted_at DESC);

CREATE INDEX idx_delivery_attempts_recipient_time
  ON delivery_attempts(recipient, attempted_at DESC);

-- +goose Down
DROP TABLE IF EXISTS delivery_attempts;
