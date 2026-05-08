-- +goose Up
ALTER TABLE companies
  ADD COLUMN parent_id uuid REFERENCES companies(id) ON DELETE SET NULL;

CREATE TABLE runtime_config (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scope_type text NOT NULL CHECK (scope_type IN ('company', 'domain', 'user')),
  scope_id uuid NOT NULL,
  key text NOT NULL,
  value jsonb NOT NULL,
  locked boolean NOT NULL DEFAULT false,
  version bigint NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(scope_type, scope_id, key)
);

CREATE INDEX runtime_config_scope_idx ON runtime_config(scope_type, scope_id);
CREATE INDEX runtime_config_key_idx ON runtime_config(key);

CREATE TABLE config_change_log (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scope_type text NOT NULL,
  scope_id uuid NOT NULL,
  key text NOT NULL,
  old_value jsonb,
  new_value jsonb NOT NULL,
  changed_by text,
  action text NOT NULL CHECK (action IN ('created', 'updated', 'deleted', 'propagated')),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX config_change_log_scope_idx ON config_change_log(scope_type, scope_id);
CREATE INDEX config_change_log_created_at_idx ON config_change_log(created_at);

-- +goose Down
DROP TABLE config_change_log;
DROP TABLE runtime_config;
ALTER TABLE companies
  DROP COLUMN parent_id;