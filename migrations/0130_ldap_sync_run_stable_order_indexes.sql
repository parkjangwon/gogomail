-- +goose Up
CREATE INDEX IF NOT EXISTS idx_ldap_sync_runs_domain_started_id
	ON ldap_sync_runs(domain_id, started_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_ldap_sync_runs_domain_status_started_id
	ON ldap_sync_runs(domain_id, status, started_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_ldap_sync_runs_success_last_success_id
	ON ldap_sync_runs(domain_id, sync_type, last_success_at DESC, id DESC)
	WHERE status = 'success';

-- +goose Down
DROP INDEX IF EXISTS idx_ldap_sync_runs_success_last_success_id;
DROP INDEX IF EXISTS idx_ldap_sync_runs_domain_status_started_id;
DROP INDEX IF EXISTS idx_ldap_sync_runs_domain_started_id;
