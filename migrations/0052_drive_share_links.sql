-- +goose Up
CREATE TABLE IF NOT EXISTS drive_share_links (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  node_id uuid NOT NULL REFERENCES drive_nodes(id) ON DELETE CASCADE,
  token_hash text NOT NULL,
  token_suffix text NOT NULL,
  permission text NOT NULL DEFAULT 'view',
  status text NOT NULL DEFAULT 'active',
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  CONSTRAINT drive_share_links_permission_check CHECK (permission IN ('view', 'download')),
  CONSTRAINT drive_share_links_status_check CHECK (status IN ('active', 'revoked')),
  CONSTRAINT drive_share_links_token_hash_check CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT drive_share_links_token_suffix_check CHECK (token_suffix <> '' AND char_length(token_suffix) <= 16),
  CONSTRAINT drive_share_links_expiry_check CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_drive_share_links_token_hash
  ON drive_share_links (token_hash);

CREATE INDEX IF NOT EXISTS idx_drive_share_links_user_node_status_updated
  ON drive_share_links (user_id, node_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_drive_share_links_expiry
  ON drive_share_links (status, expires_at)
  WHERE status = 'active';

-- +goose Down
DROP TABLE IF EXISTS drive_share_links;
