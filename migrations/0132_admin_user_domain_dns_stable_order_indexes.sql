-- +goose Up
CREATE INDEX IF NOT EXISTS idx_users_created_id
	ON users(created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_users_admin_created_id
	ON users(created_at DESC, id DESC)
	WHERE role IN ('system_admin', 'company_admin');

CREATE INDEX IF NOT EXISTS idx_domains_company_created_id
	ON domains(company_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_domains_created_id
	ON domains(created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_domain_dns_checks_domain_checked_id
	ON domain_dns_checks(domain_id, checked_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_domain_dns_checks_domain_status_checked_id
	ON domain_dns_checks(domain_id, status, checked_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_domain_dns_checks_domain_status_checked_id;
DROP INDEX IF EXISTS idx_domain_dns_checks_domain_checked_id;
DROP INDEX IF EXISTS idx_domains_created_id;
DROP INDEX IF EXISTS idx_domains_company_created_id;
DROP INDEX IF EXISTS idx_users_admin_created_id;
DROP INDEX IF EXISTS idx_users_created_id;
