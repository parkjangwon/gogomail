-- +goose Up
CREATE TABLE IF NOT EXISTS sso_configurations (
    domain_id           UUID        NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    provider            TEXT        NOT NULL CHECK (provider IN ('saml', 'oidc')),
    entity_id           TEXT,
    sso_url             TEXT,
    certificate         TEXT,
    client_id           TEXT,
    client_secret       TEXT,
    discovery_url       TEXT,
    acs_url             TEXT,
    jit_provisioning    BOOLEAN     NOT NULL DEFAULT FALSE,
    session_ttl_seconds INTEGER     NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (domain_id)
);

-- +goose Down
DROP TABLE IF EXISTS sso_configurations;
