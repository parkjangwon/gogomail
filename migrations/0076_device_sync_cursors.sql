-- +goose Up
CREATE TABLE IF NOT EXISTS device_sync_cursors (
    id          TEXT        NOT NULL,
    device_id   TEXT        NOT NULL,
    user_id     TEXT        NOT NULL,
    mailbox_id  TEXT        NOT NULL,
    version     BIGINT      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    UNIQUE (device_id, mailbox_id)
);

CREATE INDEX IF NOT EXISTS device_sync_cursors_mailbox_id_idx
    ON device_sync_cursors (mailbox_id);

-- +goose Down
DROP TABLE IF EXISTS device_sync_cursors;
