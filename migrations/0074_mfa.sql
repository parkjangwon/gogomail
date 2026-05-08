-- +goose Up
CREATE TABLE user_mfa_secrets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    secret text NOT NULL,
    recovery_codes text[] NOT NULL DEFAULT '{}',
    enabled boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id)
);

CREATE INDEX idx_user_mfa_secrets_user_id ON user_mfa_secrets(user_id);

CREATE TABLE totp_used_codes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code text NOT NULL,
    used_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, code)
);

CREATE INDEX idx_totp_used_codes_user_id ON totp_used_codes(user_id);
CREATE INDEX idx_totp_used_codes_used_at ON totp_used_codes(used_at);

-- +goose Down
DROP TABLE totp_used_codes;
DROP TABLE user_mfa_secrets;
