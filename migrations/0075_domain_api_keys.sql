-- +goose Up
CREATE TABLE domain_api_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id uuid NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    key_hash text NOT NULL UNIQUE,
    name text NOT NULL DEFAULT '',
    scopes text[] NOT NULL DEFAULT '{}',
    allowed_cidrs text[] NOT NULL DEFAULT '{}',
    expires_at timestamptz,
    revoked boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_domain_api_keys_domain_id ON domain_api_keys(domain_id);
CREATE INDEX idx_domain_api_keys_key_hash ON domain_api_keys(key_hash);

-- +goose Down
DROP TABLE domain_api_keys;
