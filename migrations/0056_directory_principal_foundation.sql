-- +goose Up
CREATE TABLE IF NOT EXISTS directory_groups (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  org_id uuid REFERENCES organizations(id) ON DELETE SET NULL,
  name text NOT NULL,
  slug text NOT NULL,
  description text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'active',
  settings jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT directory_groups_name_check CHECK (name <> '' AND char_length(name) <= 255 AND name !~ '[\r\n]'),
  CONSTRAINT directory_groups_slug_check CHECK (slug <> '' AND char_length(slug) <= 128 AND slug ~ '^[a-z0-9]([a-z0-9._-]*[a-z0-9])?$'),
  CONSTRAINT directory_groups_description_check CHECK (char_length(description) <= 2048 AND description !~ '[\r\n]'),
  CONSTRAINT directory_groups_status_check CHECK (status IN ('active', 'suspended', 'deleted'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_directory_groups_domain_active_slug
  ON directory_groups (domain_id, slug)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_directory_groups_company_status
  ON directory_groups (company_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS directory_resources (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  org_id uuid REFERENCES organizations(id) ON DELETE SET NULL,
  resource_type text NOT NULL DEFAULT 'room',
  name text NOT NULL,
  slug text NOT NULL,
  location text NOT NULL DEFAULT '',
  capacity integer NOT NULL DEFAULT 0,
  status text NOT NULL DEFAULT 'active',
  settings jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT directory_resources_type_check CHECK (resource_type IN ('room', 'equipment', 'vehicle', 'other')),
  CONSTRAINT directory_resources_name_check CHECK (name <> '' AND char_length(name) <= 255 AND name !~ '[\r\n]'),
  CONSTRAINT directory_resources_slug_check CHECK (slug <> '' AND char_length(slug) <= 128 AND slug ~ '^[a-z0-9]([a-z0-9._-]*[a-z0-9])?$'),
  CONSTRAINT directory_resources_location_check CHECK (char_length(location) <= 512 AND location !~ '[\r\n]'),
  CONSTRAINT directory_resources_capacity_check CHECK (capacity >= 0),
  CONSTRAINT directory_resources_status_check CHECK (status IN ('active', 'suspended', 'deleted'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_directory_resources_domain_active_slug
  ON directory_resources (domain_id, slug)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_directory_resources_company_status
  ON directory_resources (company_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS directory_group_memberships (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  group_id uuid NOT NULL REFERENCES directory_groups(id) ON DELETE CASCADE,
  member_kind text NOT NULL,
  member_id uuid NOT NULL,
  role text NOT NULL DEFAULT 'member',
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT directory_group_memberships_kind_check CHECK (member_kind IN ('user', 'organization', 'group', 'resource')),
  CONSTRAINT directory_group_memberships_role_check CHECK (role IN ('member', 'manager', 'owner')),
  CONSTRAINT directory_group_memberships_status_check CHECK (status IN ('active', 'deleted'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_directory_group_memberships_active_member
  ON directory_group_memberships (group_id, member_kind, member_id)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_directory_group_memberships_member
  ON directory_group_memberships (member_kind, member_id, status);

CREATE TABLE IF NOT EXISTS directory_aliases (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  alias_address text NOT NULL,
  alias_address_ace text NOT NULL,
  target_kind text NOT NULL,
  target_id uuid NOT NULL,
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT directory_aliases_address_check CHECK (alias_address <> '' AND char_length(alias_address) <= 320 AND alias_address !~ '[\r\n]'),
  CONSTRAINT directory_aliases_address_ace_check CHECK (alias_address_ace <> '' AND char_length(alias_address_ace) <= 320 AND alias_address_ace !~ '[\r\n]'),
  CONSTRAINT directory_aliases_target_kind_check CHECK (target_kind IN ('user', 'organization', 'group', 'resource')),
  CONSTRAINT directory_aliases_status_check CHECK (status IN ('active', 'deleted'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_directory_aliases_domain_active_address
  ON directory_aliases (domain_id, lower(alias_address_ace))
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_directory_aliases_target
  ON directory_aliases (target_kind, target_id, status);

-- +goose Down
DROP TABLE IF EXISTS directory_aliases;
DROP TABLE IF EXISTS directory_group_memberships;
DROP TABLE IF EXISTS directory_resources;
DROP TABLE IF EXISTS directory_groups;
