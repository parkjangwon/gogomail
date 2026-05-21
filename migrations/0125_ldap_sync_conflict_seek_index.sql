-- +goose Up
CREATE INDEX IF NOT EXISTS idx_ldap_sync_conflicts_domain_created_id
  ON ldap_sync_conflicts(domain_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_ldap_sync_conflicts_domain_unresolved_created_id
  ON ldap_sync_conflicts(domain_id, created_at DESC, id DESC)
  WHERE resolved_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ldap_sync_conflicts_run_created_id
  ON ldap_sync_conflicts(sync_run_id, created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_ldap_sync_conflicts_run_created_id;
DROP INDEX IF EXISTS idx_ldap_sync_conflicts_domain_unresolved_created_id;
DROP INDEX IF EXISTS idx_ldap_sync_conflicts_domain_created_id;
