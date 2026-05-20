-- +goose Up
-- Hot-path address lookups use address_ace for exact matching. These covering
-- indexes keep primary-address joins and per-user sender address listing stable
-- under larger tenants.
CREATE INDEX IF NOT EXISTS idx_user_addresses_user_primary_address_ace
  ON user_addresses (user_id, is_primary DESC, address_ace);

CREATE INDEX IF NOT EXISTS idx_user_addresses_user_address_ace
  ON user_addresses (user_id, address_ace);

-- +goose Down
DROP INDEX IF EXISTS idx_user_addresses_user_address_ace;
DROP INDEX IF EXISTS idx_user_addresses_user_primary_address_ace;
