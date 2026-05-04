CREATE TABLE IF NOT EXISTS api_usage_ledger_retention_runs (
  id text PRIMARY KEY,
  created_at timestamptz NOT NULL DEFAULT now(),
  cutoff timestamptz NOT NULL,
  tenant_id text NOT NULL DEFAULT '',
  principal_id text NOT NULL DEFAULT '',
  limit_count integer NOT NULL,
  dry_run boolean NOT NULL DEFAULT true,
  confirm_ready boolean NOT NULL DEFAULT false,
  ready boolean NOT NULL DEFAULT false,
  candidate_count bigint NOT NULL DEFAULT 0,
  limited_count bigint NOT NULL DEFAULT 0,
  deleted_count bigint NOT NULL DEFAULT 0,
  readiness jsonb NOT NULL DEFAULT '{}'::jsonb,
  CONSTRAINT api_usage_ledger_retention_runs_limit_check
    CHECK (limit_count > 0),
  CONSTRAINT api_usage_ledger_retention_runs_counts_check
    CHECK (candidate_count >= 0 AND limited_count >= 0 AND deleted_count >= 0),
  CONSTRAINT api_usage_ledger_retention_runs_readiness_object_check
    CHECK (jsonb_typeof(readiness) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_api_usage_ledger_retention_runs_created_at
  ON api_usage_ledger_retention_runs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_api_usage_ledger_retention_runs_tenant_created_at
  ON api_usage_ledger_retention_runs (tenant_id, created_at DESC)
  WHERE tenant_id <> '';

CREATE INDEX IF NOT EXISTS idx_api_usage_ledger_retention_runs_principal_created_at
  ON api_usage_ledger_retention_runs (principal_id, created_at DESC)
  WHERE principal_id <> '';
