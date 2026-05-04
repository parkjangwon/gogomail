CREATE TABLE IF NOT EXISTS api_usage_monthly (
  month date NOT NULL,
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
  PRIMARY KEY (month, method, route, status, user_id)
);

CREATE INDEX IF NOT EXISTS idx_api_usage_monthly_month
  ON api_usage_monthly (month DESC);

CREATE INDEX IF NOT EXISTS idx_api_usage_monthly_user_month
  ON api_usage_monthly (user_id, month DESC)
  WHERE user_id <> '';
