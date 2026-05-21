-- +goose Up
CREATE INDEX IF NOT EXISTS idx_audit_logs_company_created_id
	ON audit_logs(company_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_domain_created_id
	ON audit_logs(domain_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_created_id
	ON audit_logs(actor_id, created_at DESC, id DESC)
	WHERE actor_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_audit_logs_target_created_id
	ON audit_logs(target_id, created_at DESC, id DESC)
	WHERE target_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_audit_logs_target_created_id;
DROP INDEX IF EXISTS idx_audit_logs_actor_created_id;
DROP INDEX IF EXISTS idx_audit_logs_domain_created_id;
DROP INDEX IF EXISTS idx_audit_logs_company_created_id;
