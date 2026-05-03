-- +goose Up
ALTER TABLE domains
  ADD CONSTRAINT chk_domains_status
  CHECK (status IN ('active', 'suspended', 'disabled'));

ALTER TABLE users
  ADD CONSTRAINT chk_users_status
  CHECK (status IN ('active', 'suspended', 'disabled'));

ALTER TABLE messages
  ADD CONSTRAINT chk_messages_status
  CHECK (status IN ('active', 'draft', 'deleted')),
  ADD CONSTRAINT chk_messages_compose_intent
  CHECK (compose_intent IN ('new', 'reply', 'forward'));

-- +goose Down
ALTER TABLE messages
  DROP CONSTRAINT IF EXISTS chk_messages_compose_intent,
  DROP CONSTRAINT IF EXISTS chk_messages_status;

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS chk_users_status;

ALTER TABLE domains
  DROP CONSTRAINT IF EXISTS chk_domains_status;
