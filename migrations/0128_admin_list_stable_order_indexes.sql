-- +goose Up
CREATE INDEX IF NOT EXISTS idx_companies_created_id
  ON companies(created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_suppression_list_created_id
  ON suppression_list(created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_trusted_relays_created_id
  ON trusted_relays(created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_delivery_routes_created_id
  ON delivery_routes(created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_delivery_routes_created_id;
DROP INDEX IF EXISTS idx_trusted_relays_created_id;
DROP INDEX IF EXISTS idx_suppression_list_created_id;
DROP INDEX IF EXISTS idx_companies_created_id;
