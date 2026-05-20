-- +goose Up
CREATE INDEX IF NOT EXISTS idx_organizations_domain_status_tree_order
ON organizations (domain_id, status, depth, order_index, lower(name));

CREATE INDEX IF NOT EXISTS idx_users_org_active_display
ON users (org_id, domain_id, status, lower(display_name), id)
WHERE org_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_users_org_active_display;
DROP INDEX IF EXISTS idx_organizations_domain_status_tree_order;
