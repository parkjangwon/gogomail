CREATE TABLE IF NOT EXISTS push_devices (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  platform text NOT NULL,
  token text NOT NULL,
  label text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT push_devices_platform_check CHECK (platform IN ('apns', 'fcm', 'webpush')),
  CONSTRAINT push_devices_status_check CHECK (status IN ('active', 'deleted')),
  CONSTRAINT push_devices_label_length_check CHECK (char_length(label) <= 200),
  UNIQUE(user_id, platform, token)
);

CREATE INDEX IF NOT EXISTS idx_push_devices_user_status
  ON push_devices (user_id, status, updated_at DESC);
