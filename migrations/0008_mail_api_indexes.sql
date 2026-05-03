-- +goose Up
CREATE INDEX IF NOT EXISTS idx_folders_user_order
  ON folders(user_id, type, order_index, full_path);

CREATE INDEX IF NOT EXISTS idx_messages_user_status_time
  ON messages(user_id, status, COALESCE(received_at, created_at) DESC);

CREATE INDEX IF NOT EXISTS idx_attachments_message_created
  ON attachments(message_id, created_at, filename);

-- +goose Down
DROP INDEX IF EXISTS idx_attachments_message_created;
DROP INDEX IF EXISTS idx_messages_user_status_time;
DROP INDEX IF EXISTS idx_folders_user_order;
