-- +goose Up
CREATE TABLE IF NOT EXISTS notification_preferences (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- Global DND: applies to all folders unless overridden
    global_dnd_enabled boolean NOT NULL DEFAULT false,
    -- Schedule format (jsonb):
    -- {
    --   "weekdays": [0,1,2,3,4,5,6]   // 0=Sun, 6=Sat — days when DND active
    --   "time_ranges": [               // local-time ranges (each: HH:MM-HH:MM)
    --     {"start": "22:00", "end": "08:00"}  // ranges may cross midnight
    --   ],
    --   "timezone": "Asia/Seoul"       // IANA tz for resolving local time
    -- }
    global_dnd_schedule jsonb NOT NULL DEFAULT '{}'::jsonb,
    -- Per-folder overrides:
    -- {
    --   "<folder_id>": {
    --     "enabled": true,             // overall on/off (browser notifications)
    --     "dnd_inherit": true,         // if true, use global schedule
    --     "dnd_schedule": { ... }      // local override schedule when dnd_inherit=false
    --   }
    -- }
    folder_overrides jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notification_preferences_updated
    ON notification_preferences(updated_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_notification_preferences_updated;
DROP TABLE IF EXISTS notification_preferences;
