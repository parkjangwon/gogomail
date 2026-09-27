-- +goose Up
CREATE TABLE IF NOT EXISTS audit_retention_policies (
  company_id uuid PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
  retention_days integer NOT NULL,
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT audit_retention_policies_retention_days_check
    CHECK (retention_days IN (30, 90, 180, 365, 730))
);

CREATE TABLE IF NOT EXISTS audit_retention_runs (
  id text PRIMARY KEY,
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  retention_days integer NOT NULL,
  cutoff timestamptz NOT NULL,
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz NOT NULL DEFAULT now(),
  dry_run boolean NOT NULL DEFAULT true,
  status text NOT NULL DEFAULT 'completed',
  error_message text NOT NULL DEFAULT '',
  rows_deleted bigint NOT NULL DEFAULT 0,
  CONSTRAINT audit_retention_runs_retention_days_check
    CHECK (retention_days > 0),
  CONSTRAINT audit_retention_runs_status_check
    CHECK (status IN ('completed', 'failed')),
  CONSTRAINT audit_retention_runs_rows_deleted_check
    CHECK (rows_deleted >= 0),
  CONSTRAINT audit_retention_runs_error_message_check
    CHECK (
      length(error_message) <= 1024
      AND position(E'\n' IN error_message) = 0
      AND position(E'\r' IN error_message) = 0
    )
);

CREATE INDEX IF NOT EXISTS idx_audit_retention_runs_company_started_at
  ON audit_retention_runs (company_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_retention_runs_started_at
  ON audit_retention_runs (started_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_retention_runs;
DROP TABLE IF EXISTS audit_retention_policies;
