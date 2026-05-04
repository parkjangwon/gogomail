CREATE INDEX IF NOT EXISTS idx_api_usage_ledger_export_order
  ON api_usage_ledger (event_timestamp DESC, event_id DESC);

CREATE INDEX IF NOT EXISTS idx_api_usage_ledger_tenant_export_order
  ON api_usage_ledger (tenant_id, event_timestamp DESC, event_id DESC)
  WHERE tenant_id <> '';

CREATE INDEX IF NOT EXISTS idx_api_usage_ledger_principal_export_order
  ON api_usage_ledger (principal_id, event_timestamp DESC, event_id DESC)
  WHERE principal_id <> '';
