-- +goose Up
-- Alert Events for tracking triggered alerts

CREATE TABLE IF NOT EXISTS alert_events (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  alert_rule_id uuid NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,

  -- Current and threshold values
  current_value numeric NOT NULL,
  threshold numeric NOT NULL,

  -- Event message
  message text,

  -- Lifecycle
  triggered_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz,

  -- Index for common queries
  CONSTRAINT valid_event CHECK (triggered_at IS NOT NULL)
);

-- Indexes for fast lookups
CREATE INDEX IF NOT EXISTS idx_alert_events_company_triggered
  ON alert_events(company_id, triggered_at DESC);
CREATE INDEX IF NOT EXISTS idx_alert_events_rule_triggered
  ON alert_events(alert_rule_id, triggered_at DESC);
CREATE INDEX IF NOT EXISTS idx_alert_events_unresolved
  ON alert_events(company_id) WHERE resolved_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_alert_events_unresolved;
DROP INDEX IF EXISTS idx_alert_events_rule_triggered;
DROP INDEX IF EXISTS idx_alert_events_company_triggered;
DROP TABLE IF EXISTS alert_events;
