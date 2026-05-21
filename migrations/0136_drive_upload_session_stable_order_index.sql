-- +goose Up
CREATE INDEX IF NOT EXISTS idx_drive_upload_sessions_user_updated_created_id
	ON drive_upload_sessions(user_id, updated_at DESC, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_drive_upload_sessions_user_status_updated_created_id
	ON drive_upload_sessions(user_id, status, updated_at DESC, created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_drive_upload_sessions_user_status_updated_created_id;
DROP INDEX IF EXISTS idx_drive_upload_sessions_user_updated_created_id;
