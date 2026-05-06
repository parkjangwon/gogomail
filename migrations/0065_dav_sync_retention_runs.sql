-- +goose Up
CREATE TABLE IF NOT EXISTS dav_sync_retention_runs (
  id text PRIMARY KEY,
  created_at timestamptz NOT NULL DEFAULT now(),
  cutoff timestamptz NOT NULL,
  limit_count integer NOT NULL,
  dry_run boolean NOT NULL DEFAULT true,
  confirm_ready boolean NOT NULL DEFAULT false,
  status text NOT NULL DEFAULT 'completed',
  error_message text NOT NULL DEFAULT '',
  caldav_candidate_count bigint NOT NULL DEFAULT 0,
  caldav_deleted_count bigint NOT NULL DEFAULT 0,
  carddav_candidate_count bigint NOT NULL DEFAULT 0,
  carddav_deleted_count bigint NOT NULL DEFAULT 0,
  CONSTRAINT dav_sync_retention_runs_limit_check
    CHECK (limit_count > 0),
  CONSTRAINT dav_sync_retention_runs_status_check
    CHECK (status IN ('completed', 'failed')),
  CONSTRAINT dav_sync_retention_runs_error_message_check
    CHECK (
      length(error_message) <= 1024
      AND position(E'\n' IN error_message) = 0
      AND position(E'\r' IN error_message) = 0
    ),
  CONSTRAINT dav_sync_retention_runs_counts_check
    CHECK (
      caldav_candidate_count >= 0
      AND caldav_deleted_count >= 0
      AND carddav_candidate_count >= 0
      AND carddav_deleted_count >= 0
    )
);

CREATE INDEX IF NOT EXISTS idx_dav_sync_retention_runs_created_at
  ON dav_sync_retention_runs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_dav_sync_retention_runs_status_created_at
  ON dav_sync_retention_runs (status, created_at DESC);

-- +goose Down
