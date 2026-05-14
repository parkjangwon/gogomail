-- +goose Up
-- RDBMS Sync Metadata Tracking
-- Tracks the status, timing, and conflict resolution for per-domain RDBMS synchronization.

CREATE TABLE rdbms_sync_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL,
    sync_type TEXT NOT NULL, -- 'users', 'groups', 'memberships'
    status TEXT NOT NULL, -- 'pending', 'running', 'success', 'failed', 'partial'

    -- Sync counts
    users_created INT NOT NULL DEFAULT 0,
    users_updated INT NOT NULL DEFAULT 0,
    users_deleted INT NOT NULL DEFAULT 0,
    groups_created INT NOT NULL DEFAULT 0,
    groups_updated INT NOT NULL DEFAULT 0,
    groups_deleted INT NOT NULL DEFAULT 0,
    conflict_count INT NOT NULL DEFAULT 0,
    error_count INT NOT NULL DEFAULT 0,

    -- Timing and metadata
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms INT,

    error_message TEXT,

    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),

    CONSTRAINT fk_domain FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE,
    CONSTRAINT valid_sync_type CHECK (sync_type IN ('users', 'groups', 'memberships')),
    CONSTRAINT valid_status CHECK (status IN ('pending', 'running', 'success', 'failed', 'partial'))
);

CREATE INDEX idx_rdbms_sync_runs_domain_id ON rdbms_sync_runs(domain_id);
CREATE INDEX idx_rdbms_sync_runs_status ON rdbms_sync_runs(status);
CREATE INDEX idx_rdbms_sync_runs_started_at ON rdbms_sync_runs(started_at DESC);

-- Track last successful sync per domain per type for incremental syncs
CREATE TABLE rdbms_sync_cursors (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL UNIQUE,
    sync_type TEXT NOT NULL, -- 'users', 'groups', 'memberships'
    last_sync_time TIMESTAMP WITH TIME ZONE NOT NULL,
    query_filter TEXT, -- Query WHERE clause used for incremental sync
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),

    CONSTRAINT fk_domain FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE,
    CONSTRAINT valid_sync_type CHECK (sync_type IN ('users', 'groups', 'memberships'))
);

CREATE INDEX idx_rdbms_sync_cursors_domain_id ON rdbms_sync_cursors(domain_id);
CREATE INDEX idx_rdbms_sync_cursors_last_sync_time ON rdbms_sync_cursors(last_sync_time DESC);

-- Track conflicts for manual review
CREATE TABLE rdbms_sync_conflicts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL,
    sync_run_id uuid NOT NULL,
    conflict_type TEXT NOT NULL, -- 'duplicate_key', 'schema_mismatch', 'missing_field'
    local_data TEXT NOT NULL, -- JSON representation of local entity
    remote_data TEXT NOT NULL, -- JSON representation of RDBMS entity
    resolution TEXT, -- How it was resolved: 'prefer_local', 'prefer_rdbms', 'manual', 'deferred'
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),

    CONSTRAINT fk_domain FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE,
    CONSTRAINT fk_sync_run FOREIGN KEY (sync_run_id) REFERENCES rdbms_sync_runs(id) ON DELETE CASCADE,
    CONSTRAINT valid_conflict_type CHECK (conflict_type IN ('duplicate_key', 'schema_mismatch', 'missing_field')),
    CONSTRAINT valid_resolution CHECK (resolution IN ('prefer_local', 'prefer_rdbms', 'manual', 'deferred'))
);

CREATE INDEX idx_rdbms_sync_conflicts_domain_id ON rdbms_sync_conflicts(domain_id);
CREATE INDEX idx_rdbms_sync_conflicts_sync_run_id ON rdbms_sync_conflicts(sync_run_id);
CREATE INDEX idx_rdbms_sync_conflicts_resolved_at ON rdbms_sync_conflicts(resolved_at) WHERE resolved_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS rdbms_sync_conflicts;
DROP TABLE IF EXISTS rdbms_sync_cursors;
DROP TABLE IF EXISTS rdbms_sync_runs;
