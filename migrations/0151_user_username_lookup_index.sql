-- +goose Up
CREATE INDEX IF NOT EXISTS idx_users_local_active_username_lower
    ON users (lower(username), id)
    WHERE status = 'active' AND auth_source = 'local';

-- +goose Down
DROP INDEX IF EXISTS idx_users_local_active_username_lower;
