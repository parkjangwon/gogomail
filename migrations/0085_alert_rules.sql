-- +goose Up
-- Alert Rules for threshold-based monitoring

CREATE TABLE IF NOT EXISTS alert_rules (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,

  -- Alert configuration
  alert_type text NOT NULL
    CHECK (alert_type IN ('storage', 'login_failures', 'api_errors')),
  name text NOT NULL,
  description text,

  -- Threshold
  threshold numeric NOT NULL CHECK (threshold > 0),
  check_interval_minutes int NOT NULL DEFAULT 5 CHECK (check_interval_minutes > 0),

  -- Status
  is_enabled boolean NOT NULL DEFAULT true,

  -- Audit
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,

  CONSTRAINT valid_name CHECK (length(name) > 0)
);

-- Indexes for lookups
CREATE INDEX IF NOT EXISTS idx_alert_rules_company_type
  ON alert_rules(company_id, alert_type) WHERE is_enabled = true;
CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled
  ON alert_rules(is_enabled) WHERE is_enabled = true;

-- +goose Down
DROP INDEX IF EXISTS idx_alert_rules_enabled;
DROP INDEX IF EXISTS idx_alert_rules_company_type;
DROP TABLE IF EXISTS alert_rules;
