-- +goose Up
-- AddCardDAVACLSupport
-- Implement RFC 3744 Access Control List (ACL) support for CardDAV collections.
-- ACL rules define principal-based access control for addressbooks.

CREATE TABLE carddav_acl_rules (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  addressbook_id uuid NOT NULL,
  principal_id varchar(255) NOT NULL,
  grant_privileges text[] NOT NULL DEFAULT '{}',
  deny_privileges text[] NOT NULL DEFAULT '{}',
  protected boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT fk_carddav_acl_rules_addressbook
    FOREIGN KEY (addressbook_id)
    REFERENCES carddav_addressbooks(id) ON DELETE CASCADE,
  CONSTRAINT unique_carddav_acl_rule_per_principal
    UNIQUE(addressbook_id, principal_id)
);

CREATE INDEX idx_carddav_acl_rules_addressbook
ON carddav_acl_rules(addressbook_id);

CREATE INDEX idx_carddav_acl_rules_principal
ON carddav_acl_rules(principal_id);

COMMENT ON TABLE carddav_acl_rules IS 'RFC 3744 Access Control List rules for CardDAV addressbooks';
COMMENT ON COLUMN carddav_acl_rules.grant_privileges IS 'Array of granted privileges (read, write, delete, all)';
COMMENT ON COLUMN carddav_acl_rules.deny_privileges IS 'Array of denied privileges (overrides grants)';
COMMENT ON COLUMN carddav_acl_rules.protected IS 'Whether this ACL rule is protected from modification';

-- +goose Down
DROP TABLE IF EXISTS carddav_acl_rules;
