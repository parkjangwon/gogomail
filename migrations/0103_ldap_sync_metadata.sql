-- +goose Up
-- LDAP Sync Metadata Tracking
-- Tracks the status, timing, and conflict resolution for per-domain LDAP synchronization.

CREATE TABLE ldap_sync_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL,
    sync_type TEXT NOT NULL, -- 'users', 'groups', 'memberships'
    status TEXT NOT NULL, -- 'running', 'success', 'failed', 'partial'

    -- Sync counts
    created_count INT NOT NULL DEFAULT 0,
    updated_count INT NOT NULL DEFAULT 0,
    deleted_count INT NOT NULL DEFAULT 0,
    conflict_count INT NOT NULL DEFAULT 0,
    error_count INT NOT NULL DEFAULT 0,

    -- Conflict resolution
    resolution_strategy TEXT, -- 'prefer_local', 'prefer_ldap', 'manual_review'

    -- Timing and metadata
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    completed_at TIMESTAMP WITH TIME ZONE,
    last_success_at TIMESTAMP WITH TIME ZONE,
    duration_ms INT,

    error_message TEXT,

    CONSTRAINT fk_domain FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE,
    CONSTRAINT valid_sync_type CHECK (sync_type IN ('users', 'groups', 'memberships')),
    CONSTRAINT valid_status CHECK (status IN ('running', 'success', 'failed', 'partial'))
);

CREATE INDEX idx_ldap_sync_runs_domain_id ON ldap_sync_runs(domain_id);
CREATE INDEX idx_ldap_sync_runs_status ON ldap_sync_runs(status);
CREATE INDEX idx_ldap_sync_runs_started_at ON ldap_sync_runs(started_at DESC);

-- Track last successful sync per domain per type for incremental syncs
CREATE TABLE ldap_sync_cursors (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL UNIQUE,
    sync_type TEXT NOT NULL, -- 'users', 'groups', 'memberships'
    last_sync_time TIMESTAMP WITH TIME ZONE NOT NULL,
    filter_dn TEXT, -- DN used for incremental sync
    sync_cookie BYTEA, -- LDAP sync cookie for cookie-based incremental sync (RFC 3928)
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),

    CONSTRAINT fk_domain FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE,
    CONSTRAINT valid_sync_type CHECK (sync_type IN ('users', 'groups', 'memberships'))
);

CREATE INDEX idx_ldap_sync_cursors_domain_id ON ldap_sync_cursors(domain_id);
CREATE INDEX idx_ldap_sync_cursors_last_sync_time ON ldap_sync_cursors(last_sync_time DESC);

-- Track conflicts for manual review
CREATE TABLE ldap_sync_conflicts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL,
    sync_run_id uuid NOT NULL,
    object_type TEXT NOT NULL, -- 'user', 'group'
    object_id uuid, -- Local object ID
    ldap_dn TEXT NOT NULL, -- LDAP DN of the conflicting entry
    conflict_type TEXT NOT NULL, -- 'duplicate_key', 'missing_mapping', 'attr_mismatch'
    local_value JSONB, -- Local object data
    ldap_value JSONB, -- LDAP object data
    resolution TEXT, -- How it was resolved: 'prefer_local', 'prefer_ldap', 'manual', 'deferred'
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),

    CONSTRAINT fk_domain FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE,
    CONSTRAINT fk_sync_run FOREIGN KEY (sync_run_id) REFERENCES ldap_sync_runs(id) ON DELETE CASCADE,
    CONSTRAINT valid_object_type CHECK (object_type IN ('user', 'group')),
    CONSTRAINT valid_conflict_type CHECK (conflict_type IN ('duplicate_key', 'missing_mapping', 'attr_mismatch')),
    CONSTRAINT valid_resolution CHECK (resolution IN ('prefer_local', 'prefer_ldap', 'manual', 'deferred'))
);

CREATE INDEX idx_ldap_sync_conflicts_domain_id ON ldap_sync_conflicts(domain_id);
CREATE INDEX idx_ldap_sync_conflicts_sync_run_id ON ldap_sync_conflicts(sync_run_id);
CREATE INDEX idx_ldap_sync_conflicts_resolved_at ON ldap_sync_conflicts(resolved_at) WHERE resolved_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS ldap_sync_conflicts;
DROP TABLE IF EXISTS ldap_sync_cursors;
DROP TABLE IF EXISTS ldap_sync_runs;
