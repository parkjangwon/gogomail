-- +goose Up
-- Alert Channels for notification routing

CREATE TABLE IF NOT EXISTS alert_channels (
  id uuid PRIMARY KEY,
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,

  -- Channel type
  channel_type text NOT NULL
    CHECK (channel_type IN ('email', 'webhook', 'dashboard')),
  name text NOT NULL,

  -- Channel configuration (JSON format varies by type)
  -- email: {"recipients": ["user@example.com"]}
  -- webhook: {"url": "https://example.com/webhook", "auth_header": "Bearer token..."}
  -- dashboard: {} (no config needed)
  config jsonb NOT NULL DEFAULT '{}'::jsonb,

  -- Status
  is_enabled boolean NOT NULL DEFAULT true,

  -- Audit
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by uuid REFERENCES admin_users(id) ON DELETE SET NULL,

  CONSTRAINT valid_name CHECK (length(name) > 0)
);

-- Indexes for lookups
CREATE INDEX IF NOT EXISTS idx_alert_channels_company_type
  ON alert_channels(company_id, channel_type) WHERE is_enabled = true;

-- Alert Rule → Channel mapping (many-to-many)
CREATE TABLE IF NOT EXISTS alert_rule_channels (
  id uuid PRIMARY KEY,
  alert_rule_id uuid NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
  alert_channel_id uuid NOT NULL REFERENCES alert_channels(id) ON DELETE CASCADE,

  UNIQUE(alert_rule_id, alert_channel_id)
);

-- Indexes for lookups
CREATE INDEX IF NOT EXISTS idx_alert_rule_channels_rule
  ON alert_rule_channels(alert_rule_id);
CREATE INDEX IF NOT EXISTS idx_alert_rule_channels_channel
  ON alert_rule_channels(alert_channel_id);

-- +goose Down
DROP INDEX IF EXISTS idx_alert_rule_channels_channel;
DROP INDEX IF EXISTS idx_alert_rule_channels_rule;
DROP TABLE IF EXISTS alert_rule_channels;
DROP INDEX IF EXISTS idx_alert_channels_company_type;
DROP TABLE IF EXISTS alert_channels;
