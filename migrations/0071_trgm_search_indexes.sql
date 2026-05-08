-- Enable pg_trgm extension for fast ILIKE / fuzzy search.
-- Used by mail_flow_logs (from_addr, rcpt_to, subject) and
-- carddav_contact_objects (vcard full-text search).
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS mail_flow_logs_from_addr_trgm_idx
    ON mail_flow_logs USING GIN (from_addr gin_trgm_ops);

CREATE INDEX IF NOT EXISTS mail_flow_logs_rcpt_to_trgm_idx
    ON mail_flow_logs USING GIN (rcpt_to gin_trgm_ops);

CREATE INDEX IF NOT EXISTS mail_flow_logs_subject_trgm_idx
    ON mail_flow_logs USING GIN (subject gin_trgm_ops);

CREATE INDEX IF NOT EXISTS carddav_contact_objects_vcard_trgm_idx
    ON carddav_contact_objects USING GIN (lower(vcard::text) gin_trgm_ops);
