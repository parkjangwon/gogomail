-- +goose Up
CREATE INDEX IF NOT EXISTS idx_admin_user_roles_permanent_company_role_user
  ON admin_user_roles (company_id, role_id, user_id)
  WHERE expires_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_admin_user_roles_expiring_company_role_user
  ON admin_user_roles (company_id, role_id, expires_at, user_id)
  WHERE expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_admin_user_roles_permanent_user_company_assigned
  ON admin_user_roles (user_id, company_id, assigned_at DESC)
  WHERE expires_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_admin_user_roles_expiring_user_company_assigned
  ON admin_user_roles (user_id, company_id, expires_at, assigned_at DESC)
  WHERE expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_admin_user_roles_permanent_user_role
  ON admin_user_roles (user_id, role_id)
  WHERE expires_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_admin_user_roles_expiring_user_role
  ON admin_user_roles (user_id, expires_at, role_id)
  WHERE expires_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_admin_user_roles_expiring_user_role;
DROP INDEX IF EXISTS idx_admin_user_roles_permanent_user_role;
DROP INDEX IF EXISTS idx_admin_user_roles_expiring_user_company_assigned;
DROP INDEX IF EXISTS idx_admin_user_roles_permanent_user_company_assigned;
DROP INDEX IF EXISTS idx_admin_user_roles_expiring_company_role_user;
DROP INDEX IF EXISTS idx_admin_user_roles_permanent_company_role_user;
