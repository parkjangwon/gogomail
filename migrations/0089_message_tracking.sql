-- +goose Up
-- message_tracking_pixels: one row per (message, recipient) when tracking is enabled
CREATE TABLE message_tracking_pixels (
  pixel_id TEXT PRIMARY KEY,
  message_id UUID NOT NULL,
  sender_user_id UUID NOT NULL,
  recipient_email TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_tracking_pixels_message ON message_tracking_pixels(message_id);
CREATE INDEX idx_tracking_pixels_sender ON message_tracking_pixels(sender_user_id, message_id);

-- message_tracking_events: recorded each time the pixel is loaded
CREATE TABLE message_tracking_events (
  id BIGSERIAL PRIMARY KEY,
  pixel_id TEXT NOT NULL REFERENCES message_tracking_pixels(pixel_id) ON DELETE CASCADE,
  opened_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ip TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_tracking_events_pixel ON message_tracking_events(pixel_id);

-- +goose Down
DROP TABLE IF EXISTS message_tracking_events;
DROP TABLE IF EXISTS message_tracking_pixels;
