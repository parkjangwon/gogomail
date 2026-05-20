-- +goose Up
CREATE INDEX IF NOT EXISTS idx_messages_user_folder_thread_key_active_message_at
  ON messages(
    user_id,
    folder_id,
    COALESCE(thread_id, id),
    COALESCE(received_at, sent_at, draft_updated_at, created_at) DESC,
    id DESC
  )
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_messages_user_thread_key_active_message_at
  ON messages(
    user_id,
    COALESCE(thread_id, id),
    COALESCE(received_at, sent_at, draft_updated_at, created_at) DESC,
    id DESC
  )
  WHERE status = 'active';

-- +goose Down
DROP INDEX IF EXISTS idx_messages_user_thread_key_active_message_at;
DROP INDEX IF EXISTS idx_messages_user_folder_thread_key_active_message_at;
