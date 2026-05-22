-- +goose Up
-- Alert configuration and notification channels
-- Supersedes 0085_alert_rules, 0086_alert_channels, 0087_alert_events with a redesigned schema.

DROP TABLE IF EXISTS alert_events CASCADE;
DROP TABLE IF EXISTS alert_rule_channels CASCADE;
DROP TABLE IF EXISTS alert_channels CASCADE;
DROP TABLE IF EXISTS alert_rules CASCADE;

CREATE TABLE IF NOT EXISTS alert_configs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,

  -- Alert type and threshold
  alert_type text NOT NULL
    CHECK (alert_type IN ('storage', 'login_failures', 'api_errors')),
  threshold numeric NOT NULL CHECK (threshold > 0),
  name text NOT NULL,
  description text,

  -- Check interval
  check_interval_minutes int NOT NULL DEFAULT 5 CHECK (check_interval_minutes > 0),

  -- Status
  is_enabled boolean NOT NULL DEFAULT true,

  -- Audit
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,

  CONSTRAINT valid_name CHECK (length(name) > 0)
);

CREATE INDEX IF NOT EXISTS idx_alert_configs_company_type
  ON alert_configs(company_id, alert_type) WHERE is_enabled = true;
CREATE INDEX IF NOT EXISTS idx_alert_configs_company
  ON alert_configs(company_id);

CREATE TABLE IF NOT EXISTS alert_channels (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  alert_config_id uuid NOT NULL REFERENCES alert_configs(id) ON DELETE CASCADE,

  -- Channel type
  channel_type text NOT NULL
    CHECK (channel_type IN ('email', 'webhook', 'dashboard')),

  -- Configuration (email: address, webhook: url, dashboard: always enabled)
  config jsonb NOT NULL DEFAULT '{}',

  -- Status
  is_enabled boolean NOT NULL DEFAULT true,

  -- Audit
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alert_channels_config
  ON alert_channels(alert_config_id) WHERE is_enabled = true;

CREATE TABLE IF NOT EXISTS alert_notifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  alert_config_id uuid NOT NULL REFERENCES alert_configs(id) ON DELETE CASCADE,

  -- Alert details
  alert_type text NOT NULL,
  threshold numeric NOT NULL,
  current_value numeric NOT NULL,

  -- Channel notification status
  email_sent boolean DEFAULT false,
  webhook_sent boolean DEFAULT false,
  dashboard_shown boolean DEFAULT false,

  -- Metadata
  notification_data jsonb,

  -- Audit
  created_at timestamptz NOT NULL DEFAULT now(),
  acknowledged_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_alert_notifications_company_time
  ON alert_notifications(company_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alert_notifications_config
  ON alert_notifications(alert_config_id);

-- +goose Down
-- Drop the new (0105) schema in reverse dependency order.
DROP INDEX IF EXISTS idx_alert_notifications_config;
DROP INDEX IF EXISTS idx_alert_notifications_company_time;
DROP TABLE IF EXISTS alert_notifications;
DROP INDEX IF EXISTS idx_alert_channels_config;
DROP TABLE IF EXISTS alert_channels;
DROP INDEX IF EXISTS idx_alert_configs_company;
DROP INDEX IF EXISTS idx_alert_configs_company_type;
DROP TABLE IF EXISTS alert_configs;

-- Recreate the original schema from 0085_alert_rules, 0086_alert_channels, 0087_alert_events
-- so rolling back 0105 restores the pre-0105 alert system rather than leaving it empty.

-- From 0085_alert_rules.sql
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

CREATE INDEX IF NOT EXISTS idx_alert_rules_company_type
  ON alert_rules(company_id, alert_type) WHERE is_enabled = true;
CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled
  ON alert_rules(is_enabled) WHERE is_enabled = true;

-- From 0086_alert_channels.sql
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
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,

  CONSTRAINT valid_name CHECK (length(name) > 0)
);

CREATE INDEX IF NOT EXISTS idx_alert_channels_company_type
  ON alert_channels(company_id, channel_type) WHERE is_enabled = true;

CREATE TABLE IF NOT EXISTS alert_rule_channels (
  id uuid PRIMARY KEY,
  alert_rule_id uuid NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
  alert_channel_id uuid NOT NULL REFERENCES alert_channels(id) ON DELETE CASCADE,

  UNIQUE(alert_rule_id, alert_channel_id)
);

CREATE INDEX IF NOT EXISTS idx_alert_rule_channels_rule
  ON alert_rule_channels(alert_rule_id);
CREATE INDEX IF NOT EXISTS idx_alert_rule_channels_channel
  ON alert_rule_channels(alert_channel_id);

-- From 0087_alert_events.sql
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

  CONSTRAINT valid_event CHECK (triggered_at IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_alert_events_company_triggered
  ON alert_events(company_id, triggered_at DESC);
CREATE INDEX IF NOT EXISTS idx_alert_events_rule_triggered
  ON alert_events(alert_rule_id, triggered_at DESC);
CREATE INDEX IF NOT EXISTS idx_alert_events_unresolved
  ON alert_events(company_id) WHERE resolved_at IS NULL;
