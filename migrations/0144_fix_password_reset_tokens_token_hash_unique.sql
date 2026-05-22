-- +goose Up
-- Fix: password_reset_tokens.token_hash has no unique index, allowing duplicate token hashes.
CREATE UNIQUE INDEX password_reset_tokens_token_hash_idx ON password_reset_tokens(token_hash);

-- +goose Down
DROP INDEX IF EXISTS password_reset_tokens_token_hash_idx;
