-- +goose Up
CREATE INDEX IF NOT EXISTS idx_companies_quota_pressure_updated_id
	ON companies(
		((quota_used::double precision / quota_limit::double precision)) DESC,
		updated_at DESC,
		id DESC
	)
	WHERE quota_limit IS NOT NULL AND quota_limit > 0;

CREATE INDEX IF NOT EXISTS idx_domains_quota_pressure_updated_id
	ON domains(
		((quota_used::double precision / quota_limit::double precision)) DESC,
		updated_at DESC,
		id DESC
	)
	WHERE quota_limit IS NOT NULL AND quota_limit > 0;

CREATE INDEX IF NOT EXISTS idx_domains_quota_pressure_domain_updated_id
	ON domains(
		id,
		((quota_used::double precision / quota_limit::double precision)) DESC,
		updated_at DESC
	)
	WHERE quota_limit IS NOT NULL AND quota_limit > 0;

CREATE INDEX IF NOT EXISTS idx_users_quota_pressure_updated_id
	ON users(
		((quota_used::double precision / quota_limit::double precision)) DESC,
		updated_at DESC,
		id DESC
	)
	WHERE quota_limit IS NOT NULL AND quota_limit > 0;

CREATE INDEX IF NOT EXISTS idx_users_quota_pressure_domain_updated_id
	ON users(
		domain_id,
		((quota_used::double precision / quota_limit::double precision)) DESC,
		updated_at DESC,
		id DESC
	)
	WHERE quota_limit IS NOT NULL AND quota_limit > 0;

-- +goose Down
DROP INDEX IF EXISTS idx_users_quota_pressure_domain_updated_id;
DROP INDEX IF EXISTS idx_users_quota_pressure_updated_id;
DROP INDEX IF EXISTS idx_domains_quota_pressure_domain_updated_id;
DROP INDEX IF EXISTS idx_domains_quota_pressure_updated_id;
DROP INDEX IF EXISTS idx_companies_quota_pressure_updated_id;
