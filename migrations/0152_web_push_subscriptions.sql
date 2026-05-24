-- +goose Up
CREATE TABLE IF NOT EXISTS web_push_subscriptions (
  id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  endpoint   TEXT        NOT NULL,
  p256dh     TEXT        NOT NULL,
  auth       TEXT        NOT NULL,
  user_agent TEXT,
  status     TEXT        NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_web_push_subscriptions_endpoint_active
  ON web_push_subscriptions(endpoint)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_web_push_subscriptions_user_active
  ON web_push_subscriptions(user_id)
  WHERE status = 'active';

-- +goose Down
DROP INDEX IF EXISTS idx_web_push_subscriptions_user_active;
DROP INDEX IF EXISTS idx_web_push_subscriptions_endpoint_active;
DROP TABLE IF EXISTS web_push_subscriptions;
