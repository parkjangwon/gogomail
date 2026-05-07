-- +goose Up
CREATE TABLE IF NOT EXISTS attachment_share_links (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  attachment_id uuid NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
  token_hash text NOT NULL,
  token_suffix text NOT NULL,
  permission text NOT NULL DEFAULT 'download',
  status text NOT NULL DEFAULT 'active',
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  CONSTRAINT attachment_share_links_permission_check CHECK (permission IN ('download')),
  CONSTRAINT attachment_share_links_status_check CHECK (status IN ('active', 'revoked')),
  CONSTRAINT attachment_share_links_token_hash_check CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT attachment_share_links_token_suffix_check CHECK (token_suffix <> '' AND char_length(token_suffix) <= 16),
  CONSTRAINT attachment_share_links_expiry_check CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_attachment_share_links_token_hash
  ON attachment_share_links (token_hash);

CREATE INDEX IF NOT EXISTS idx_attachment_share_links_user_attachment_status_updated
  ON attachment_share_links (user_id, attachment_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_attachment_share_links_expiry
  ON attachment_share_links (status, expires_at)
  WHERE status = 'active';

-- +goose Down
DROP TABLE IF EXISTS attachment_share_links;
