-- +goose Up
CREATE TABLE IF NOT EXISTS directory_delegations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  owner_kind text NOT NULL,
  owner_id uuid NOT NULL,
  delegate_kind text NOT NULL,
  delegate_id uuid NOT NULL,
  scope text NOT NULL,
  role text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT directory_delegations_owner_kind_check CHECK (owner_kind IN ('user', 'organization', 'group', 'resource')),
  CONSTRAINT directory_delegations_delegate_kind_check CHECK (delegate_kind IN ('user', 'organization', 'group', 'resource')),
  CONSTRAINT directory_delegations_scope_check CHECK (scope IN ('calendar', 'contacts', 'drive', 'mailbox')),
  CONSTRAINT directory_delegations_role_check CHECK (role IN ('read', 'write', 'manage')),
  CONSTRAINT directory_delegations_status_check CHECK (status IN ('active', 'deleted')),
  CONSTRAINT directory_delegations_no_self_check CHECK (owner_kind <> delegate_kind OR owner_id <> delegate_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_directory_delegations_active_grant
  ON directory_delegations (company_id, owner_kind, owner_id, delegate_kind, delegate_id, scope, role)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_directory_delegations_delegate_lookup
  ON directory_delegations (company_id, delegate_kind, delegate_id, scope, status);

CREATE INDEX IF NOT EXISTS idx_directory_delegations_owner_lookup
  ON directory_delegations (company_id, owner_kind, owner_id, scope, status);

-- +goose Down
DROP TABLE IF EXISTS directory_delegations;
