DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_ledger_status_check'
  ) THEN
    ALTER TABLE api_usage_ledger
      ADD CONSTRAINT api_usage_ledger_status_check
      CHECK (status BETWEEN 100 AND 599);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_ledger_request_count_check'
  ) THEN
    ALTER TABLE api_usage_ledger
      ADD CONSTRAINT api_usage_ledger_request_count_check
      CHECK (request_count > 0);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_ledger_bytes_latency_check'
  ) THEN
    ALTER TABLE api_usage_ledger
      ADD CONSTRAINT api_usage_ledger_bytes_latency_check
      CHECK (
        request_bytes >= 0
        AND response_bytes >= 0
        AND latency_ms >= 0
      );
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'api_usage_ledger_payload_object_check'
  ) THEN
    ALTER TABLE api_usage_ledger
      ADD CONSTRAINT api_usage_ledger_payload_object_check
      CHECK (jsonb_typeof(payload) = 'object');
  END IF;
END $$;
