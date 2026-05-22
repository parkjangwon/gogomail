-- +goose Up
-- Fix: admin_audit_logs.admin_user_id is NOT NULL but has ON DELETE SET NULL — contradiction.
-- Remove NOT NULL constraint so SET NULL can work when the referenced user is deleted.
ALTER TABLE admin_audit_logs ALTER COLUMN admin_user_id DROP NOT NULL;

-- +goose Down
-- Restore NOT NULL only if no existing rows have NULL (will fail if NULLs are present).
ALTER TABLE admin_audit_logs ALTER COLUMN admin_user_id SET NOT NULL;
