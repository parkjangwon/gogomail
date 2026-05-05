-- +goose Up
CREATE TABLE IF NOT EXISTS drive_object_cleanup_failures (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  node_id uuid REFERENCES drive_nodes(id) ON DELETE SET NULL,
  storage_backend text NOT NULL,
  storage_path text NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  attempts integer NOT NULL DEFAULT 1,
  last_error text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz,
  CONSTRAINT drive_object_cleanup_failures_status_check CHECK (status IN ('pending', 'resolved')),
  CONSTRAINT drive_object_cleanup_failures_attempts_check CHECK (attempts > 0),
  CONSTRAINT drive_object_cleanup_failures_storage_backend_check CHECK (storage_backend <> '' AND char_length(storage_backend) <= 64),
  CONSTRAINT drive_object_cleanup_failures_storage_path_check CHECK (storage_path <> '' AND char_length(storage_path) <= 1024),
  CONSTRAINT drive_object_cleanup_failures_last_error_check CHECK (last_error <> '' AND char_length(last_error) <= 2048)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_drive_object_cleanup_failures_pending_object
  ON drive_object_cleanup_failures (storage_backend, storage_path)
  WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_drive_object_cleanup_failures_user_status_updated
  ON drive_object_cleanup_failures (user_id, status, updated_at DESC);

-- +goose Down
DROP TABLE IF EXISTS drive_object_cleanup_failures;
