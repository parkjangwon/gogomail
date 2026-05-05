-- +goose Up
DROP INDEX IF EXISTS idx_directory_aliases_domain_active_address;

CREATE UNIQUE INDEX IF NOT EXISTS idx_directory_aliases_active_address
  ON directory_aliases (lower(alias_address_ace))
  WHERE status = 'active';

-- +goose Down
DROP INDEX IF EXISTS idx_directory_aliases_active_address;

CREATE UNIQUE INDEX IF NOT EXISTS idx_directory_aliases_domain_active_address
  ON directory_aliases (domain_id, lower(alias_address_ace))
  WHERE status = 'active';
