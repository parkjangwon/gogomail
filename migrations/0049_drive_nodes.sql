-- +goose Up
CREATE TABLE IF NOT EXISTS drive_nodes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  parent_id uuid REFERENCES drive_nodes(id) ON DELETE CASCADE,
  node_type text NOT NULL,
  name text NOT NULL,
  normalized_name text NOT NULL,
  mime_type text NOT NULL DEFAULT '',
  size bigint NOT NULL DEFAULT 0,
  storage_backend text NOT NULL DEFAULT '',
  storage_path text NOT NULL DEFAULT '',
  checksum_sha256 text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'active',
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  trashed_at timestamptz,
  deleted_at timestamptz,
  CONSTRAINT drive_nodes_type_check CHECK (node_type IN ('folder', 'file')),
  CONSTRAINT drive_nodes_status_check CHECK (status IN ('active', 'trashed', 'deleted')),
  CONSTRAINT drive_nodes_name_check CHECK (name <> '' AND char_length(name) <= 255),
  CONSTRAINT drive_nodes_normalized_name_check CHECK (normalized_name <> '' AND char_length(normalized_name) <= 255),
  CONSTRAINT drive_nodes_size_check CHECK (size >= 0),
  CONSTRAINT drive_nodes_storage_backend_check CHECK (storage_backend = '' OR char_length(storage_backend) <= 64),
  CONSTRAINT drive_nodes_storage_path_check CHECK (char_length(storage_path) <= 1024),
  CONSTRAINT drive_nodes_checksum_check CHECK (checksum_sha256 = '' OR checksum_sha256 ~ '^[0-9a-f]{64}$'),
  CONSTRAINT drive_nodes_metadata_object_check CHECK (jsonb_typeof(metadata) = 'object'),
  CONSTRAINT drive_nodes_folder_storage_check CHECK (
    (node_type = 'folder' AND size = 0 AND storage_backend = '' AND storage_path = '' AND checksum_sha256 = '')
    OR
    (node_type = 'file' AND storage_backend <> '' AND storage_path <> '')
  )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_drive_nodes_user_active_sibling_name
  ON drive_nodes (user_id, COALESCE(parent_id, '00000000-0000-0000-0000-000000000000'::uuid), normalized_name)
  WHERE status = 'active';

CREATE UNIQUE INDEX IF NOT EXISTS idx_drive_nodes_storage_object
  ON drive_nodes (storage_backend, storage_path)
  WHERE storage_path <> '';

CREATE INDEX IF NOT EXISTS idx_drive_nodes_user_parent_status_updated
  ON drive_nodes (user_id, parent_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_drive_nodes_user_status_updated
  ON drive_nodes (user_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_drive_nodes_domain_status_updated
  ON drive_nodes (domain_id, status, updated_at DESC);

-- +goose Down
DROP TABLE IF EXISTS drive_nodes;
