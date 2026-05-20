-- +goose Up
CREATE INDEX idx_organization_members_active_unit_primary_created
  ON organization_members (organization_unit_id, is_primary DESC, created_at)
  WHERE ended_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_organization_members_active_unit_primary_created;
