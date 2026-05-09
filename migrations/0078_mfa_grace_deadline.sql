-- +goose Up
ALTER TABLE user_mfa_secrets
    ADD COLUMN IF NOT EXISTS mfa_grace_deadline TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_user_mfa_secrets_grace_deadline
    ON user_mfa_secrets (mfa_grace_deadline)
    WHERE mfa_grace_deadline IS NOT NULL AND enabled = FALSE;

-- +goose Down
DROP INDEX IF EXISTS idx_user_mfa_secrets_grace_deadline;
ALTER TABLE user_mfa_secrets DROP COLUMN IF EXISTS mfa_grace_deadline;
