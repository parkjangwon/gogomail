-- +goose Up

-- Add flag to force account setup (username + password) on first login
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS requires_initial_setup BOOLEAN NOT NULL DEFAULT false;

-- Add index for users needing initial setup
CREATE INDEX IF NOT EXISTS idx_users_requires_initial_setup
  ON users(requires_initial_setup) WHERE requires_initial_setup = true;

-- Add constraint to allow non-email usernames only for admin role
ALTER TABLE users
  ADD CONSTRAINT admin_or_email_address
    CHECK (
      role = 'system_admin'
      OR address ~ '^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
    );

-- Create default bootstrap company if not exists
-- +goose StatementBegin
DO $$
DECLARE
  default_company_id uuid;
  default_domain_id uuid;
BEGIN
  -- Create default company
  INSERT INTO companies (name, status)
    VALUES ('System Administrator', 'active')
    ON CONFLICT DO NOTHING
    RETURNING id INTO default_company_id;

  -- Get or create default company
  SELECT id INTO default_company_id FROM companies
    WHERE name = 'System Administrator' LIMIT 1;

  -- Create default domain
  INSERT INTO domains (company_id, name, name_ace, status)
    VALUES (default_company_id, 'system', 'system', 'active')
    ON CONFLICT (name) DO NOTHING
    RETURNING id INTO default_domain_id;

  -- Get or create default domain
  SELECT id INTO default_domain_id FROM domains
    WHERE name = 'system' LIMIT 1;

  -- Create bootstrap admin user (requires initial setup of username + password)
  INSERT INTO users (
    domain_id, username, display_name, password_hash,
    auth_source, role, status, address, requires_initial_setup
  ) VALUES (
    default_domain_id, 'admin', 'System Administrator',
    'plain:admin1234', 'local', 'system_admin', 'active',
    'admin', true
  )
  ON CONFLICT (domain_id, username) DO UPDATE
    SET requires_initial_setup = true;
END;
$$;
-- +goose StatementEnd

-- +goose Down

-- Remove constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS admin_or_email_address;

-- Remove index
DROP INDEX IF EXISTS idx_users_requires_initial_setup;

-- Remove column
ALTER TABLE users DROP COLUMN IF EXISTS requires_initial_setup;

-- Delete bootstrap data
DELETE FROM users WHERE username = 'admin' AND role = 'system_admin';
DELETE FROM domains WHERE name = 'system';
DELETE FROM companies WHERE name = 'System Administrator';
