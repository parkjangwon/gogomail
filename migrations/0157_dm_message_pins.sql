-- +goose Up
CREATE TABLE IF NOT EXISTS dm_message_pins (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id uuid NOT NULL REFERENCES dm_rooms(id) ON DELETE CASCADE,
  message_id uuid NOT NULL REFERENCES dm_messages(id) ON DELETE CASCADE,
  pinned_by_user_id uuid NOT NULL REFERENCES users(id),
  note text,
  pinned_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT dm_message_pins_note_check CHECK (note IS NULL OR char_length(note) <= 500),
  UNIQUE (room_id, message_id)
);

CREATE INDEX IF NOT EXISTS idx_dm_message_pins_room_pinned_desc
  ON dm_message_pins (room_id, pinned_at DESC);

-- +goose Down
DROP TABLE IF EXISTS dm_message_pins;
