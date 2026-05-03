-- +goose Up
ALTER TABLE messages
  ADD COLUMN compose_intent text NOT NULL DEFAULT 'new',
  ADD COLUMN source_message_id uuid REFERENCES messages(id) ON DELETE SET NULL,
  ADD COLUMN draft_updated_at timestamptz;

ALTER TABLE attachments
  ADD COLUMN user_id uuid REFERENCES users(id) ON DELETE CASCADE,
  ADD COLUMN draft_id uuid REFERENCES messages(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX idx_attachments_upload_id ON attachments(upload_id);
CREATE INDEX idx_messages_user_drafts
  ON messages(user_id, draft_updated_at DESC, id DESC)
  WHERE status = 'draft';
CREATE INDEX idx_messages_user_source
  ON messages(user_id, source_message_id)
  WHERE source_message_id IS NOT NULL;
CREATE INDEX idx_attachments_user_draft
  ON attachments(user_id, draft_id, created_at)
  WHERE draft_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_attachments_user_draft;
DROP INDEX IF EXISTS idx_messages_user_source;
DROP INDEX IF EXISTS idx_messages_user_drafts;
DROP INDEX IF EXISTS idx_attachments_upload_id;

ALTER TABLE attachments
  DROP COLUMN IF EXISTS draft_id,
  DROP COLUMN IF EXISTS user_id;

ALTER TABLE messages
  DROP COLUMN IF EXISTS draft_updated_at,
  DROP COLUMN IF EXISTS source_message_id,
  DROP COLUMN IF EXISTS compose_intent;
