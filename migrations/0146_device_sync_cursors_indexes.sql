-- +goose Up
-- device_sync_cursors stores user_id and mailbox_id as TEXT (see 0076),
-- while users.id and folders.id are UUID. Adding a FK requires changing
-- column types with USING uuid casts, which is risky if any historical
-- rows contain non-UUID text values. Instead, add supporting indexes
-- for cleanup queries and document the limitation.
--
-- Cleanup queries typically run:
--   DELETE FROM device_sync_cursors WHERE user_id = $1;
--   DELETE FROM device_sync_cursors WHERE user_id = $1 AND mailbox_id = $2;
-- so we add an index on user_id and a composite index on (user_id, mailbox_id).
-- A FK migration should be performed only after operational verification
-- that all existing rows are valid UUID text (see follow-up task).

CREATE INDEX IF NOT EXISTS device_sync_cursors_user_id_idx
    ON device_sync_cursors (user_id);

CREATE INDEX IF NOT EXISTS device_sync_cursors_user_mailbox_idx
    ON device_sync_cursors (user_id, mailbox_id);

-- +goose Down
DROP INDEX IF EXISTS device_sync_cursors_user_mailbox_idx;
DROP INDEX IF EXISTS device_sync_cursors_user_id_idx;
