-- +goose Up
-- Add constraints to api_usage_ledger table
ALTER TABLE api_usage_ledger
  ADD CONSTRAINT api_usage_ledger_status_check
    CHECK (status BETWEEN 100 AND 599);

ALTER TABLE api_usage_ledger
  ADD CONSTRAINT api_usage_ledger_request_count_check
    CHECK (request_count > 0);

ALTER TABLE api_usage_ledger
  ADD CONSTRAINT api_usage_ledger_bytes_latency_check
    CHECK (
      request_bytes >= 0
      AND response_bytes >= 0
      AND latency_ms >= 0
    );

ALTER TABLE api_usage_ledger
  ADD CONSTRAINT api_usage_ledger_payload_object_check
    CHECK (jsonb_typeof(payload) = 'object');

-- +goose Down
ALTER TABLE api_usage_ledger DROP CONSTRAINT IF EXISTS api_usage_ledger_status_check;
ALTER TABLE api_usage_ledger DROP CONSTRAINT IF EXISTS api_usage_ledger_request_count_check;
ALTER TABLE api_usage_ledger DROP CONSTRAINT IF EXISTS api_usage_ledger_bytes_latency_check;
ALTER TABLE api_usage_ledger DROP CONSTRAINT IF EXISTS api_usage_ledger_payload_object_check;
