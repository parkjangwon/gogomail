-- +goose Up
ALTER TABLE drive_share_links
  ADD COLUMN IF NOT EXISTS password_hash text NOT NULL DEFAULT '';

ALTER TABLE drive_share_links
  DROP CONSTRAINT IF EXISTS drive_share_links_password_hash_check;

ALTER TABLE drive_share_links
  ADD CONSTRAINT drive_share_links_password_hash_check
  CHECK (password_hash = '' OR (char_length(password_hash) <= 4096 AND password_hash !~ '[\r\n]'));

-- +goose Down
ALTER TABLE drive_share_links
  DROP CONSTRAINT IF EXISTS drive_share_links_password_hash_check;

ALTER TABLE drive_share_links
  DROP COLUMN IF EXISTS password_hash;
