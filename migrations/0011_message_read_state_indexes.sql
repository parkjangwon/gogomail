-- +goose Up
CREATE INDEX idx_messages_user_folder_active_at
  ON messages(user_id, folder_id, COALESCE(received_at, sent_at, draft_updated_at, created_at) DESC, id DESC)
  WHERE status = 'active';

CREATE INDEX idx_messages_user_folder_unread
  ON messages(user_id, folder_id)
  WHERE status = 'active'
    AND COALESCE((flags->>'read')::boolean, false) = false;

CREATE INDEX idx_messages_user_folder_starred
  ON messages(user_id, folder_id)
  WHERE status = 'active'
    AND COALESCE((flags->>'starred')::boolean, false) = true;

-- +goose Down
DROP INDEX IF EXISTS idx_messages_user_folder_starred;
DROP INDEX IF EXISTS idx_messages_user_folder_unread;
DROP INDEX IF EXISTS idx_messages_user_folder_active_at;
