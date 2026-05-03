-- +goose Up
CREATE TABLE dkim_keys (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  selector text NOT NULL,
  private_key_pem text NOT NULL,
  public_key_dns text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(domain_id, selector)
);

CREATE INDEX idx_dkim_keys_domain_status
  ON dkim_keys(domain_id, status, updated_at DESC);

-- +goose Down
DROP TABLE IF EXISTS dkim_keys;
