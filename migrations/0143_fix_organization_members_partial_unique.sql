-- +goose Up
-- Fix: organization_members has no partial unique index for active memberships,
-- allowing duplicate active memberships for the same (unit, user) pair.
CREATE UNIQUE INDEX organization_members_active_unique
    ON organization_members(organization_unit_id, user_id)
    WHERE ended_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS organization_members_active_unique;
