-- +goose Up
CREATE TABLE delivery_routes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  domain_pattern text NOT NULL,
  farm text NOT NULL DEFAULT '',
  hosts text[] NOT NULL,
  port integer NOT NULL DEFAULT 25,
  tls_mode text NOT NULL DEFAULT 'opportunistic',
  implicit_tls boolean NOT NULL DEFAULT false,
  smtp_hello text NOT NULL DEFAULT '',
  pool_name text NOT NULL DEFAULT '',
  auth_identity text NOT NULL DEFAULT '',
  auth_username text NOT NULL DEFAULT '',
  auth_password text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'active',
  description text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT delivery_routes_domain_pattern_not_blank CHECK (btrim(domain_pattern) <> ''),
  CONSTRAINT delivery_routes_hosts_not_empty CHECK (cardinality(hosts) > 0),
  CONSTRAINT delivery_routes_port_valid CHECK (port BETWEEN 1 AND 65535),
  CONSTRAINT delivery_routes_tls_mode_valid CHECK (tls_mode IN ('opportunistic', 'require', 'disable')),
  CONSTRAINT delivery_routes_status_valid CHECK (status IN ('active', 'disabled'))
);

CREATE UNIQUE INDEX idx_delivery_routes_domain_pattern_unique
  ON delivery_routes(lower(domain_pattern));

CREATE INDEX idx_delivery_routes_status_created_at
  ON delivery_routes(status, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS delivery_routes;
