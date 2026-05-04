-- +goose Up
CREATE TABLE IF NOT EXISTS push_notification_attempts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  message_id uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  rfc_message_id text NOT NULL DEFAULT '',
  company_id uuid REFERENCES companies(id) ON DELETE SET NULL,
  domain_id uuid REFERENCES domains(id) ON DELETE SET NULL,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  recipient text NOT NULL DEFAULT '',
  subject text NOT NULL DEFAULT '',
  device_id uuid REFERENCES push_devices(id) ON DELETE SET NULL,
  platform text NOT NULL,
  token_suffix text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'candidate',
  error_message text NOT NULL DEFAULT '',
  attempted_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT push_notification_attempts_platform_check CHECK (platform IN ('apns', 'fcm', 'webpush')),
  CONSTRAINT push_notification_attempts_status_check CHECK (status IN ('candidate', 'queued', 'delivered', 'failed', 'invalid_token')),
  CONSTRAINT push_notification_attempts_subject_length_check CHECK (char_length(subject) <= 500),
  CONSTRAINT push_notification_attempts_error_length_check CHECK (char_length(error_message) <= 2000)
);

CREATE INDEX IF NOT EXISTS idx_push_notification_attempts_user_time
  ON push_notification_attempts (user_id, attempted_at DESC);

CREATE INDEX IF NOT EXISTS idx_push_notification_attempts_message_time
  ON push_notification_attempts (message_id, attempted_at DESC);

CREATE INDEX IF NOT EXISTS idx_push_notification_attempts_device_time
  ON push_notification_attempts (device_id, attempted_at DESC);

-- +goose Down
