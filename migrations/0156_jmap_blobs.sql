-- +goose Up
CREATE TABLE IF NOT EXISTS jmap_blobs (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    storage_path TEXT        NOT NULL,
    content_type TEXT        NOT NULL DEFAULT 'application/octet-stream',
    size         BIGINT      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '7 days'
);

CREATE INDEX IF NOT EXISTS jmap_blobs_account_id_idx ON jmap_blobs(account_id);
CREATE INDEX IF NOT EXISTS jmap_blobs_expires_at_idx ON jmap_blobs(expires_at);

-- +goose Down
DROP TABLE IF EXISTS jmap_blobs;
