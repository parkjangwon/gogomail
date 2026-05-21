-- +goose Up
CREATE INDEX IF NOT EXISTS idx_dkim_keys_domain_status_updated_id
  ON dkim_keys(domain_id, status, updated_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_dkim_keys_updated_id
  ON dkim_keys(updated_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_dkim_keys_updated_id;
DROP INDEX IF EXISTS idx_dkim_keys_domain_status_updated_id;
