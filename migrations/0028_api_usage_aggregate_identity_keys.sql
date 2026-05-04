-- +goose Up
ALTER TABLE api_usage_daily
  DROP CONSTRAINT IF EXISTS api_usage_daily_pkey;

ALTER TABLE api_usage_daily
  ADD PRIMARY KEY (
    day,
    method,
    route,
    status,
    tenant_id,
    company_id,
    domain_id,
    user_id,
    api_key_id,
    principal_id,
    auth_source
  );

ALTER TABLE api_usage_monthly
  DROP CONSTRAINT IF EXISTS api_usage_monthly_pkey;

ALTER TABLE api_usage_monthly
  ADD PRIMARY KEY (
    month,
    method,
    route,
    status,
    tenant_id,
    company_id,
    domain_id,
    user_id,
    api_key_id,
    principal_id,
    auth_source
  );

CREATE INDEX IF NOT EXISTS idx_api_usage_daily_principal_day
  ON api_usage_daily (principal_id, day DESC)
  WHERE principal_id <> '';

CREATE INDEX IF NOT EXISTS idx_api_usage_monthly_principal_month
  ON api_usage_monthly (principal_id, month DESC)
  WHERE principal_id <> '';

CREATE INDEX IF NOT EXISTS idx_api_usage_daily_tenant_day
  ON api_usage_daily (tenant_id, day DESC)
  WHERE tenant_id <> '';

CREATE INDEX IF NOT EXISTS idx_api_usage_monthly_tenant_month
  ON api_usage_monthly (tenant_id, month DESC)
  WHERE tenant_id <> '';

-- +goose Down
