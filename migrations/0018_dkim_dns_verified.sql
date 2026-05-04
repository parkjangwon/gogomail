-- +goose Up
ALTER TABLE dkim_keys ADD COLUMN dns_verified_at timestamptz;

-- +goose Down
ALTER TABLE dkim_keys DROP COLUMN dns_verified_at;
