CREATE TABLE IF NOT EXISTS api_usage_events (
  event_id text PRIMARY KEY,
  event_timestamp timestamptz NOT NULL,
  method text NOT NULL,
  route text NOT NULL,
  status integer NOT NULL,
  user_id text NOT NULL DEFAULT '',
  first_seen_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_api_usage_events_timestamp
  ON api_usage_events (event_timestamp DESC);
