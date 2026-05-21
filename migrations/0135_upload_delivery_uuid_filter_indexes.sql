-- +goose Up
CREATE INDEX IF NOT EXISTS idx_attachment_upload_sessions_user_created_id
	ON attachment_upload_sessions(user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_delivery_attempts_message_attempted_id
	ON delivery_attempts(message_id, attempted_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_delivery_attempts_message_status_attempted_id
	ON delivery_attempts(message_id, status, attempted_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_delivery_attempts_message_status_attempted_id;
DROP INDEX IF EXISTS idx_delivery_attempts_message_attempted_id;
DROP INDEX IF EXISTS idx_attachment_upload_sessions_user_created_id;
