-- +goose Up
CREATE TABLE quota_alert_thresholds (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scope text NOT NULL CHECK (scope IN ('user', 'domain', 'company')),
  scope_id uuid,
  company_id uuid NOT NULL,
  warning_ratio double precision NOT NULL DEFAULT 0.8 CHECK (warning_ratio > 0 AND warning_ratio <= 1),
  critical_ratio double precision NOT NULL DEFAULT 0.95 CHECK (critical_ratio > 0 AND critical_ratio <= 1),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (scope, scope_id)
);

CREATE INDEX idx_quota_alert_thresholds_company ON quota_alert_thresholds(company_id);
CREATE INDEX idx_quota_alert_thresholds_scope ON quota_alert_thresholds(scope, scope_id);

CREATE TABLE quota_alerts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  domain_id uuid,
  user_id uuid,
  scope text NOT NULL CHECK (scope IN ('user', 'domain', 'company')),
  alert_type text NOT NULL CHECK (alert_type IN ('warning', 'critical', 'exhausted')),
  quota_used bigint NOT NULL,
  quota_limit bigint NOT NULL,
  usage_ratio double precision NOT NULL,
  event_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_quota_alerts_company_time ON quota_alerts(company_id, created_at DESC);
CREATE INDEX idx_quota_alerts_domain_time ON quota_alerts(domain_id, created_at DESC);
CREATE INDEX idx_quota_alerts_user_time ON quota_alerts(user_id, created_at DESC);
CREATE INDEX idx_quota_alerts_scope ON quota_alerts(scope, scope_id, created_at DESC);
CREATE INDEX idx_quota_alerts_event_id ON quota_alerts(event_id);

-- +goose Down
DROP INDEX IF EXISTS idx_quota_alerts_event_id;
DROP INDEX IF EXISTS idx_quota_alerts_scope;
DROP INDEX IF EXISTS idx_quota_alerts_user_time;
DROP INDEX IF EXISTS idx_quota_alerts_domain_time;
DROP INDEX IF EXISTS idx_quota_alerts_company_time;
DROP TABLE IF EXISTS quota_alerts;
DROP INDEX IF EXISTS idx_quota_alert_thresholds_scope;
DROP INDEX IF EXISTS idx_quota_alert_thresholds_company;
DROP TABLE IF EXISTS quota_alert_thresholds;