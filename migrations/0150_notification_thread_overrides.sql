-- +goose Up
ALTER TABLE notification_preferences
    ADD COLUMN IF NOT EXISTS thread_overrides jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE notification_preferences
    DROP COLUMN IF EXISTS thread_overrides;
