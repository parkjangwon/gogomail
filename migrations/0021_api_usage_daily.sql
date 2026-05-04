CREATE TABLE IF NOT EXISTS api_usage_daily (
  day date NOT NULL,
  method text NOT NULL,
  route text NOT NULL,
  status integer NOT NULL,
  user_id text NOT NULL DEFAULT '',
  request_count bigint NOT NULL DEFAULT 0,
  request_bytes bigint NOT NULL DEFAULT 0,
  response_bytes bigint NOT NULL DEFAULT 0,
  latency_ms_total bigint NOT NULL DEFAULT 0,
  latency_ms_max bigint NOT NULL DEFAULT 0,
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (day, method, route, status, user_id)
);

CREATE INDEX IF NOT EXISTS idx_api_usage_daily_day
  ON api_usage_daily (day DESC);

CREATE INDEX IF NOT EXISTS idx_api_usage_daily_user_day
  ON api_usage_daily (user_id, day DESC)
  WHERE user_id <> '';
