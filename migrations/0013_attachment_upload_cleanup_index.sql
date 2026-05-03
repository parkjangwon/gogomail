-- +goose Up
-- +goose NO TRANSACTION
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_attachments_uploading_created_at
  ON attachments (created_at ASC, id ASC)
  WHERE status = 'uploading';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_attachments_uploading_created_at;
