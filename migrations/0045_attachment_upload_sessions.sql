-- +goose Up
CREATE TABLE IF NOT EXISTS attachment_upload_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  draft_id uuid REFERENCES messages(id) ON DELETE SET NULL,
  upload_id text NOT NULL,
  filename text NOT NULL,
  declared_size bigint NOT NULL,
  received_size bigint NOT NULL DEFAULT 0,
  mime_type text NOT NULL,
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
  CONSTRAINT attachment_upload_sessions_status_check CHECK (status IN ('pending', 'uploading', 'complete', 'canceled', 'expired', 'failed')),
  CONSTRAINT attachment_upload_sessions_size_check CHECK (declared_size >= 0 AND received_size >= 0 AND received_size <= declared_size),
  CONSTRAINT attachment_upload_sessions_filename_check CHECK (filename <> '' AND char_length(filename) <= 255),
  CONSTRAINT attachment_upload_sessions_mime_type_check CHECK (mime_type <> '' AND char_length(mime_type) <= 255),
  CONSTRAINT attachment_upload_sessions_storage_backend_check CHECK (storage_backend <> '' AND char_length(storage_backend) <= 64),
  CONSTRAINT attachment_upload_sessions_checksum_check CHECK (checksum_sha256 = '' OR checksum_sha256 ~ '^[0-9a-f]{64}$'),
  CONSTRAINT attachment_upload_sessions_metadata_object_check CHECK (jsonb_typeof(adapter_metadata) = 'object')
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_attachment_upload_sessions_user_upload
  ON attachment_upload_sessions (user_id, upload_id);

CREATE INDEX IF NOT EXISTS idx_attachment_upload_sessions_user_created
  ON attachment_upload_sessions (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_attachment_upload_sessions_expiry
  ON attachment_upload_sessions (status, expires_at)
  WHERE status IN ('pending', 'uploading', 'failed');

-- +goose Down
DROP TABLE IF EXISTS attachment_upload_sessions;
