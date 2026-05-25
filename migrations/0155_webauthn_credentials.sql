-- +goose Up
-- WebAuthn credentials for users (Passkey/FIDO2 MFA)
CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id BYTEA NOT NULL,             -- WebAuthn credential ID (raw bytes)
    public_key    BYTEA NOT NULL,             -- CBOR-encoded public key
    aaguid        UUID,                       -- Authenticator AAGUID
    sign_count    BIGINT NOT NULL DEFAULT 0,
    name          TEXT NOT NULL DEFAULT '',   -- User-assigned friendly name
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at  TIMESTAMPTZ,
    UNIQUE (user_id, credential_id)
);

-- Challenge store (short-lived, can be cleaned up with TTL)
CREATE TABLE IF NOT EXISTS webauthn_challenges (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    challenge   BYTEA NOT NULL,              -- Raw challenge bytes
    flow        TEXT NOT NULL,               -- 'registration' or 'authentication'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL DEFAULT now() + interval '5 minutes'
);
CREATE INDEX IF NOT EXISTS webauthn_challenges_user_flow ON webauthn_challenges(user_id, flow);
CREATE INDEX IF NOT EXISTS webauthn_challenges_expires ON webauthn_challenges(expires_at);

-- +goose Down
DROP TABLE IF EXISTS webauthn_challenges;
DROP TABLE IF EXISTS webauthn_credentials;
