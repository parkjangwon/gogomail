-- +goose Up
CREATE TABLE IF NOT EXISTS drive_upload_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  parent_id uuid REFERENCES drive_nodes(id) ON DELETE SET NULL,
  upload_id text NOT NULL,
  name text NOT NULL,
  declared_size bigint NOT NULL,
  received_size bigint NOT NULL DEFAULT 0,
  mime_type text NOT NULL DEFAULT 'application/octet-stream',
  status text NOT NULL DEFAULT 'pending',
  storage_backend text NOT NULL DEFAULT 'local',
  storage_path text NOT NULL DEFAULT '',
  checksum_sha256 text NOT NULL DEFAULT '',
  adapter_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  finalized_at timestamptz,
  canceled_at timestamptz,
  CONSTRAINT drive_upload_sessions_status_check CHECK (status IN ('pending', 'uploading', 'finalized', 'canceled', 'expired', 'failed')),
  CONSTRAINT drive_upload_sessions_size_check CHECK (declared_size >= 0 AND received_size >= 0 AND received_size <= declared_size),
  CONSTRAINT drive_upload_sessions_name_check CHECK (name <> '' AND char_length(name) <= 255),
  CONSTRAINT drive_upload_sessions_mime_type_check CHECK (mime_type <> '' AND char_length(mime_type) <= 255),
  CONSTRAINT drive_upload_sessions_storage_backend_check CHECK (storage_backend <> '' AND char_length(storage_backend) <= 64),
  CONSTRAINT drive_upload_sessions_checksum_check CHECK (checksum_sha256 = '' OR checksum_sha256 ~ '^[0-9a-f]{64}$'),
  CONSTRAINT drive_upload_sessions_metadata_object_check CHECK (jsonb_typeof(adapter_metadata) = 'object')
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_drive_upload_sessions_user_upload
  ON drive_upload_sessions (user_id, upload_id);

CREATE INDEX IF NOT EXISTS idx_drive_upload_sessions_user_created
  ON drive_upload_sessions (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_drive_upload_sessions_parent_created
  ON drive_upload_sessions (user_id, parent_id, created_at DESC)
  WHERE status IN ('pending', 'uploading', 'failed');

CREATE INDEX IF NOT EXISTS idx_drive_upload_sessions_expiry
  ON drive_upload_sessions (status, expires_at)
  WHERE status IN ('pending', 'uploading', 'failed');

-- +goose Down
DROP TABLE IF EXISTS drive_upload_sessions;
