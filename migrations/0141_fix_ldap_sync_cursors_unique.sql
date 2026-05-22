-- +goose Up
-- Fix: ldap_sync_cursors has UNIQUE(domain_id) but the table also has sync_type,
-- so the unique key should be (domain_id, sync_type) to allow one cursor per type per domain.
ALTER TABLE ldap_sync_cursors DROP CONSTRAINT ldap_sync_cursors_domain_id_key;
ALTER TABLE ldap_sync_cursors ADD CONSTRAINT ldap_sync_cursors_domain_id_sync_type_key UNIQUE (domain_id, sync_type);

-- +goose Down
ALTER TABLE ldap_sync_cursors DROP CONSTRAINT ldap_sync_cursors_domain_id_sync_type_key;
ALTER TABLE ldap_sync_cursors ADD CONSTRAINT ldap_sync_cursors_domain_id_key UNIQUE (domain_id);
