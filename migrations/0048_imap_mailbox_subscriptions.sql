-- +goose Up
CREATE TABLE IF NOT EXISTS imap_mailbox_subscriptions (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  mailbox_name text NOT NULL,
  canonical_name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, canonical_name),
  CHECK (mailbox_name <> ''),
  CHECK (canonical_name <> '')
);

CREATE INDEX IF NOT EXISTS idx_imap_mailbox_subscriptions_user_name
  ON imap_mailbox_subscriptions (user_id, mailbox_name);

-- +goose Down
