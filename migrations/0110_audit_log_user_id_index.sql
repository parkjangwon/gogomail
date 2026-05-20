-- +goose Up
-- Add index for user_id filtering on audit_logs.
-- Without this, queries filtered by user_id perform a full table scan
-- as the existing indexes only cover (company_id, created_at) and (actor_id, created_at).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_user_time
    ON audit_logs (user_id, created_at DESC, id DESC)
    WHERE user_id IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_audit_logs_user_time;
