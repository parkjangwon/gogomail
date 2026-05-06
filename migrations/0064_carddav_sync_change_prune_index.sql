-- +goose Up
CREATE INDEX IF NOT EXISTS idx_carddav_addressbook_changes_prune
  ON carddav_addressbook_changes (changed_at, id);

-- +goose Down
DROP INDEX IF EXISTS idx_carddav_addressbook_changes_prune;
