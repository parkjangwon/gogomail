-- +goose Up
CREATE TABLE trusted_relays (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  cidr inet NOT NULL UNIQUE,
  description text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_trusted_relays_created_at
  ON trusted_relays(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS trusted_relays;
