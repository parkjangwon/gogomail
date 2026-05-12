-- +goose Up
CREATE INDEX IF NOT EXISTS idx_carddav_addressbook_changes_user_book_token
  ON carddav_addressbook_changes (user_id, addressbook_id, sync_token);

CREATE INDEX IF NOT EXISTS idx_carddav_addressbook_changes_user_book_id_covering
  ON carddav_addressbook_changes (user_id, addressbook_id, id)
  INCLUDE (object_name, etag, action, sync_token, changed_at);

-- +goose Down
DROP INDEX IF EXISTS idx_carddav_addressbook_changes_user_book_id_covering;
DROP INDEX IF EXISTS idx_carddav_addressbook_changes_user_book_token;
