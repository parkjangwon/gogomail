-- +goose Up
CREATE TABLE IF NOT EXISTS imap_mailbox_state (
  mailbox_id uuid PRIMARY KEY REFERENCES folders(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  uidvalidity integer NOT NULL DEFAULT (1 + floor(random() * 2147483646))::integer,
  uidnext bigint NOT NULL DEFAULT 1,
  highest_modseq bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT imap_mailbox_state_uidvalidity_check CHECK (uidvalidity > 0),
  CONSTRAINT imap_mailbox_state_uidnext_check CHECK (uidnext > 0 AND uidnext <= 4294967295),
  CONSTRAINT imap_mailbox_state_highest_modseq_check CHECK (highest_modseq > 0)
);

CREATE INDEX IF NOT EXISTS idx_imap_mailbox_state_user
  ON imap_mailbox_state (user_id);

CREATE TABLE IF NOT EXISTS imap_message_uid (
  message_id uuid PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
  mailbox_id uuid NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  uid bigint NOT NULL,
  modseq bigint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT imap_message_uid_uid_check CHECK (uid > 0 AND uid <= 4294967295),
  CONSTRAINT imap_message_uid_modseq_check CHECK (modseq > 0),
  UNIQUE (mailbox_id, uid)
);

CREATE INDEX IF NOT EXISTS idx_imap_message_uid_mailbox_uid
  ON imap_message_uid (mailbox_id, uid);

CREATE INDEX IF NOT EXISTS idx_imap_message_uid_user_mailbox_modseq
  ON imap_message_uid (user_id, mailbox_id, modseq);

-- +goose Down
