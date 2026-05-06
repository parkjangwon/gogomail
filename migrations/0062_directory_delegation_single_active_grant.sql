-- +goose Up
DROP INDEX IF EXISTS idx_directory_delegations_active_grant;

CREATE UNIQUE INDEX IF NOT EXISTS idx_directory_delegations_active_grant
  ON directory_delegations (company_id, owner_kind, owner_id, delegate_kind, delegate_id, scope)
  WHERE status = 'active';

-- +goose Down
DROP INDEX IF EXISTS idx_directory_delegations_active_grant;

CREATE UNIQUE INDEX IF NOT EXISTS idx_directory_delegations_active_grant
  ON directory_delegations (company_id, owner_kind, owner_id, delegate_kind, delegate_id, scope, role)
  WHERE status = 'active';
