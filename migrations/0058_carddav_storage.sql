-- +goose Up
CREATE TABLE IF NOT EXISTS carddav_addressbooks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  normalized_name text NOT NULL,
  description text NOT NULL DEFAULT '',
  sync_token text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT carddav_addressbooks_name_check CHECK (name <> '' AND char_length(name) <= 255 AND name !~ '[\r\n]'),
  CONSTRAINT carddav_addressbooks_normalized_name_check CHECK (normalized_name <> '' AND char_length(normalized_name) <= 255 AND normalized_name !~ '[\r\n]'),
  CONSTRAINT carddav_addressbooks_description_check CHECK (char_length(description) <= 2048 AND description !~ '[\r\n]'),
  CONSTRAINT carddav_addressbooks_sync_token_check CHECK (sync_token <> '' AND char_length(sync_token) <= 128),
  CONSTRAINT carddav_addressbooks_status_check CHECK (status IN ('active', 'deleted'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_carddav_addressbooks_user_active_name
  ON carddav_addressbooks (user_id, normalized_name)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_carddav_addressbooks_user_status_updated
  ON carddav_addressbooks (user_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS carddav_contact_objects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  addressbook_id uuid NOT NULL REFERENCES carddav_addressbooks(id) ON DELETE CASCADE,
  object_name text NOT NULL,
  uid text NOT NULL,
  etag text NOT NULL,
  size bigint NOT NULL,
  vcard text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT carddav_contact_objects_name_check CHECK (object_name <> '' AND char_length(object_name) <= 200 AND lower(object_name) LIKE '%.vcf'),
  CONSTRAINT carddav_contact_objects_uid_check CHECK (uid <> '' AND char_length(uid) <= 255 AND uid !~ '[\r\n]'),
  CONSTRAINT carddav_contact_objects_etag_check CHECK (etag ~ '^"[0-9a-f]{64}"$'),
  CONSTRAINT carddav_contact_objects_size_check CHECK (size >= 0 AND size <= 5242880),
  CONSTRAINT carddav_contact_objects_vcard_check CHECK (vcard <> ''),
  CONSTRAINT carddav_contact_objects_status_check CHECK (status IN ('active', 'deleted'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_carddav_contact_objects_active_name
  ON carddav_contact_objects (addressbook_id, object_name)
  WHERE status = 'active';

CREATE UNIQUE INDEX IF NOT EXISTS idx_carddav_contact_objects_active_uid
  ON carddav_contact_objects (addressbook_id, uid)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_carddav_contact_objects_user_book_updated
  ON carddav_contact_objects (user_id, addressbook_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS carddav_addressbook_changes (
  id bigserial PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  addressbook_id uuid NOT NULL REFERENCES carddav_addressbooks(id) ON DELETE CASCADE,
  object_name text NOT NULL DEFAULT '',
  etag text NOT NULL DEFAULT '',
  action text NOT NULL,
  sync_token text NOT NULL,
  changed_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT carddav_addressbook_changes_object_name_check CHECK (char_length(object_name) <= 200 AND object_name !~ '[\r\n]'),
  CONSTRAINT carddav_addressbook_changes_etag_check CHECK (etag = '' OR etag ~ '^"[0-9a-f]{64}"$'),
  CONSTRAINT carddav_addressbook_changes_action_check CHECK (action IN ('addressbook-created', 'addressbook-updated', 'contact-upserted', 'contact-deleted', 'addressbook-deleted')),
  CONSTRAINT carddav_addressbook_changes_sync_token_check CHECK (sync_token <> '' AND char_length(sync_token) <= 128)
);

CREATE INDEX IF NOT EXISTS idx_carddav_addressbook_changes_book_id
  ON carddav_addressbook_changes (addressbook_id, id);

-- +goose Down
DROP TABLE IF EXISTS carddav_addressbook_changes;
DROP TABLE IF EXISTS carddav_contact_objects;
DROP TABLE IF EXISTS carddav_addressbooks;
