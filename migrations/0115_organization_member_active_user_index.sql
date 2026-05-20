-- +goose Up
CREATE INDEX idx_organization_members_active_user_unit
  ON organization_members (user_id, is_primary DESC, organization_unit_id)
  WHERE ended_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_organization_members_active_user_unit;
