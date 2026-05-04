-- +goose Up
CREATE TABLE domain_dns_checks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  status text NOT NULL,
  report jsonb NOT NULL,
  checked_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT domain_dns_checks_status_valid CHECK (status IN ('ok', 'missing', 'mismatch', 'error'))
);

CREATE INDEX idx_domain_dns_checks_domain_time
  ON domain_dns_checks(domain_id, checked_at DESC);

-- +goose Down
DROP TABLE IF EXISTS domain_dns_checks;
