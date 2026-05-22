-- +goose Up
-- Fix: message_tracking_pixels.message_id and sender_user_id have no FK constraints.
ALTER TABLE message_tracking_pixels
    ADD CONSTRAINT fk_mtp_message
        FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE;

ALTER TABLE message_tracking_pixels
    ADD CONSTRAINT fk_mtp_user
        FOREIGN KEY (sender_user_id) REFERENCES users(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE message_tracking_pixels DROP CONSTRAINT IF EXISTS fk_mtp_user;
ALTER TABLE message_tracking_pixels DROP CONSTRAINT IF EXISTS fk_mtp_message;
