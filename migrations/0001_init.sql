-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE companies (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  quota_used bigint NOT NULL DEFAULT 0,
  quota_limit bigint,
  settings jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE domains (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  name text NOT NULL UNIQUE,
  name_ace text NOT NULL UNIQUE,
  status text NOT NULL DEFAULT 'active',
  quota_used bigint NOT NULL DEFAULT 0,
  quota_limit bigint,
  settings jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE organizations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  parent_id uuid REFERENCES organizations(id) ON DELETE SET NULL,
  name text NOT NULL,
  code text NOT NULL DEFAULT '',
  depth int NOT NULL DEFAULT 0,
  order_index int NOT NULL DEFAULT 0,
  status text NOT NULL DEFAULT 'active',
  settings jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  org_id uuid REFERENCES organizations(id) ON DELETE SET NULL,
  username text NOT NULL,
  display_name text NOT NULL,
  recovery_email text NOT NULL DEFAULT '',
  password_hash text,
  auth_source text NOT NULL DEFAULT 'local',
  role text NOT NULL DEFAULT 'user',
  status text NOT NULL DEFAULT 'active',
  quota_used bigint NOT NULL DEFAULT 0,
  quota_limit bigint,
  settings jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(domain_id, username)
);

CREATE TABLE user_addresses (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  local_part text NOT NULL,
  local_part_ace text NOT NULL,
  domain_ace text NOT NULL,
  address text NOT NULL UNIQUE,
  address_ace text NOT NULL UNIQUE,
  is_primary boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE folders (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  parent_id uuid REFERENCES folders(id) ON DELETE CASCADE,
  name text NOT NULL,
  full_path text NOT NULL,
  type text NOT NULL DEFAULT 'user',
  system_type text,
  order_index int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(user_id, full_path)
);

CREATE TABLE messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  folder_id uuid NOT NULL REFERENCES folders(id) ON DELETE RESTRICT,
  rfc_message_id text,
  in_reply_to text,
  thread_id uuid,
  subject text NOT NULL DEFAULT '',
  from_addr text NOT NULL DEFAULT '',
  from_name text NOT NULL DEFAULT '',
  to_addrs jsonb NOT NULL DEFAULT '[]'::jsonb,
  cc_addrs jsonb NOT NULL DEFAULT '[]'::jsonb,
  bcc_addrs jsonb NOT NULL DEFAULT '[]'::jsonb,
  received_at timestamptz,
  sent_at timestamptz,
  size bigint NOT NULL DEFAULT 0,
  has_attachment boolean NOT NULL DEFAULT false,
  flags jsonb NOT NULL DEFAULT '{"read":false,"starred":false,"answered":false,"forwarded":false}'::jsonb,
  spam_score double precision,
  storage_path text NOT NULL,
  dek_encrypted bytea,
  status text NOT NULL DEFAULT 'active',
  deleted_at timestamptz,
  legal_hold boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_messages_user_folder ON messages(user_id, folder_id, received_at DESC);
CREATE INDEX idx_messages_thread ON messages(user_id, thread_id);
CREATE INDEX idx_messages_rfc_id ON messages(domain_id, rfc_message_id);
CREATE INDEX idx_messages_flags ON messages USING gin(flags);

CREATE TABLE attachments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  message_id uuid REFERENCES messages(id) ON DELETE CASCADE,
  upload_id text NOT NULL,
  storage_path text NOT NULL,
  filename text NOT NULL,
  size bigint NOT NULL,
  mime_type text NOT NULL,
  status text NOT NULL DEFAULT 'uploading',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE outbox (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  topic text NOT NULL,
  partition_key text NOT NULL,
  payload jsonb NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  attempts int NOT NULL DEFAULT 0,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz
);

CREATE INDEX idx_outbox_pending ON outbox(status, created_at);

CREATE TABLE audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid,
  domain_id uuid,
  user_id uuid,
  actor_id uuid,
  category text NOT NULL,
  action text NOT NULL,
  target_type text NOT NULL DEFAULT '',
  target_id uuid,
  ip_address inet,
  user_agent text NOT NULL DEFAULT '',
  result text NOT NULL,
  detail jsonb NOT NULL DEFAULT '{}'::jsonb,
  prev_hash text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_scope_time ON audit_logs(company_id, domain_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS folders;
DROP TABLE IF EXISTS user_addresses;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;
DROP TABLE IF EXISTS domains;
DROP TABLE IF EXISTS companies;
