-- +goose Up
CREATE TABLE IF NOT EXISTS dm_rooms (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  room_type text NOT NULL CHECK (room_type IN ('direct','group')),
  visibility text CHECK (visibility IN ('public','private')),
  name text,
  owner_id uuid REFERENCES users(id),
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT dm_rooms_direct_shape_check CHECK (
    (room_type = 'direct' AND visibility IS NULL AND name IS NULL AND owner_id IS NULL)
    OR
    (room_type = 'group' AND visibility IN ('public','private') AND owner_id IS NOT NULL)
  ),
  CONSTRAINT dm_rooms_group_name_check CHECK (room_type <> 'group' OR char_length(trim(name)) BETWEEN 1 AND 120)
);

CREATE TABLE IF NOT EXISTS dm_room_keys (
  room_id uuid PRIMARY KEY REFERENCES dm_rooms(id) ON DELETE CASCADE,
  key_ciphertext bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS dm_participants (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id uuid NOT NULL REFERENCES dm_rooms(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id),
  joined_at timestamptz NOT NULL DEFAULT now(),
  last_read_message_id uuid,
  UNIQUE (room_id, user_id)
);

CREATE TABLE IF NOT EXISTS dm_messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id uuid NOT NULL REFERENCES dm_rooms(id) ON DELETE CASCADE,
  sender_id uuid REFERENCES users(id),
  message_type text NOT NULL CHECK (message_type IN ('text','file','drive_link','system')),
  body bytea NOT NULL,
  attachment_storage_path bytea,
  attachment_name text,
  attachment_size bigint,
  attachment_mime_type text,
  drive_file_id uuid REFERENCES drive_nodes(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  edited_at timestamptz,
  deleted_at timestamptz,
  CONSTRAINT dm_messages_attachment_size_check CHECK (attachment_size IS NULL OR attachment_size >= 0)
);

CREATE TABLE IF NOT EXISTS dm_reactions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  message_id uuid NOT NULL REFERENCES dm_messages(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id),
  emoji text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (message_id, user_id, emoji)
);

CREATE TABLE IF NOT EXISTS dm_message_urls (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  message_id uuid NOT NULL REFERENCES dm_messages(id) ON DELETE CASCADE,
  room_id uuid NOT NULL REFERENCES dm_rooms(id) ON DELETE CASCADE,
  url text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS dm_invites (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id uuid NOT NULL REFERENCES dm_rooms(id) ON DELETE CASCADE,
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  used_by uuid REFERENCES users(id),
  CONSTRAINT dm_invites_expiry_check CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS idx_dm_rooms_user_listing
  ON dm_participants (user_id, joined_at DESC, room_id);

CREATE INDEX IF NOT EXISTS idx_dm_rooms_domain_public
  ON dm_rooms (domain_id, created_at DESC)
  WHERE room_type = 'group' AND visibility = 'public';

CREATE INDEX IF NOT EXISTS idx_dm_messages_room_created_desc
  ON dm_messages (room_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_dm_messages_room_id_desc
  ON dm_messages (room_id, id DESC);

CREATE INDEX IF NOT EXISTS idx_dm_reactions_message
  ON dm_reactions (message_id);

CREATE INDEX IF NOT EXISTS idx_dm_message_urls_room_created_desc
  ON dm_message_urls (room_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_dm_invites_room
  ON dm_invites (room_id);

-- +goose Down
DROP TABLE IF EXISTS dm_invites;
DROP TABLE IF EXISTS dm_message_urls;
DROP TABLE IF EXISTS dm_reactions;
DROP TABLE IF EXISTS dm_messages;
DROP TABLE IF EXISTS dm_participants;
DROP TABLE IF EXISTS dm_room_keys;
DROP TABLE IF EXISTS dm_rooms;
