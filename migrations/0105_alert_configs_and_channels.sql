-- +goose Up
-- Alert configuration and notification channels

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
DROP INDEX IF EXISTS idx_alert_notifications_config;
DROP INDEX IF EXISTS idx_alert_notifications_company_time;
DROP TABLE IF EXISTS alert_notifications;
DROP INDEX IF EXISTS idx_alert_channels_config;
DROP TABLE IF EXISTS alert_channels;
DROP INDEX IF EXISTS idx_alert_configs_company;
DROP INDEX IF EXISTS idx_alert_configs_company_type;
DROP TABLE IF EXISTS alert_configs;
