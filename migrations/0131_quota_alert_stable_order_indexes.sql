-- +goose Up
CREATE INDEX IF NOT EXISTS idx_quota_alert_thresholds_company_created_id
	ON quota_alert_thresholds(company_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_quota_alert_thresholds_company_scope_created_id
	ON quota_alert_thresholds(company_id, scope, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_quota_alert_thresholds_scope_lookup_created_id
	ON quota_alert_thresholds(company_id, scope, scope_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_quota_alerts_company_created_id
	ON quota_alerts(company_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_quota_alerts_domain_created_id
	ON quota_alerts(domain_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_quota_alerts_user_created_id
	ON quota_alerts(user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_quota_alerts_company_scope_type_created_id
	ON quota_alerts(company_id, scope, alert_type, created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_quota_alerts_company_scope_type_created_id;
DROP INDEX IF EXISTS idx_quota_alerts_user_created_id;
DROP INDEX IF EXISTS idx_quota_alerts_domain_created_id;
DROP INDEX IF EXISTS idx_quota_alerts_company_created_id;
DROP INDEX IF EXISTS idx_quota_alert_thresholds_scope_lookup_created_id;
DROP INDEX IF EXISTS idx_quota_alert_thresholds_company_scope_created_id;
DROP INDEX IF EXISTS idx_quota_alert_thresholds_company_created_id;
