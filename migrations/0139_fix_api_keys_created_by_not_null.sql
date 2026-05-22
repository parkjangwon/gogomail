-- +goose Up
-- Fix: api_keys.created_by is NOT NULL but has ON DELETE SET NULL — contradiction.
-- Remove NOT NULL constraint so SET NULL can work when the referenced user is deleted.
ALTER TABLE api_keys ALTER COLUMN created_by DROP NOT NULL;

-- +goose Down
-- Restore NOT NULL only if no existing rows have NULL (will fail if NULLs are present).
ALTER TABLE api_keys ALTER COLUMN created_by SET NOT NULL;
