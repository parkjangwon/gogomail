-- +goose Up
-- +goose StatementBegin

-- Admin role definitions (builtin + custom)
CREATE TABLE admin_role_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    is_builtin BOOLEAN NOT NULL DEFAULT false,
    created_by UUID,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT fk_admin_role_company FOREIGN KEY (company_id)
        REFERENCES companies(id) ON DELETE CASCADE,
    CONSTRAINT unique_admin_role_name UNIQUE (company_id, name)
);

CREATE INDEX idx_admin_role_company ON admin_role_definitions(company_id);
CREATE INDEX idx_admin_role_builtin ON admin_role_definitions(is_builtin);

-- Admin role permissions (resource × action × scope)
CREATE TABLE admin_role_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL,
    resource TEXT NOT NULL,
    action TEXT NOT NULL,
    scope TEXT NOT NULL,
    conditions JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_permission_role FOREIGN KEY (role_id)
        REFERENCES admin_role_definitions(id) ON DELETE CASCADE,
    CONSTRAINT unique_role_permission UNIQUE (role_id, resource, action, scope)
);

CREATE INDEX idx_permission_role ON admin_role_permissions(role_id);
CREATE INDEX idx_permission_resource ON admin_role_permissions(resource);

-- User-role assignments
CREATE TABLE admin_user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    user_id UUID NOT NULL,
    role_id UUID NOT NULL,
    assigned_at TIMESTAMP NOT NULL DEFAULT NOW(),
    assigned_by UUID,
    expires_at TIMESTAMP,

    CONSTRAINT fk_user_role_company FOREIGN KEY (company_id)
        REFERENCES companies(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_role_user FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_role_role FOREIGN KEY (role_id)
        REFERENCES admin_role_definitions(id) ON DELETE CASCADE,
    CONSTRAINT unique_user_role UNIQUE (company_id, user_id, role_id)
);

CREATE INDEX idx_user_role_company ON admin_user_roles(company_id);
CREATE INDEX idx_user_role_user ON admin_user_roles(user_id);
CREATE INDEX idx_user_role_role ON admin_user_roles(role_id);
CREATE INDEX idx_user_role_expires ON admin_user_roles(expires_at) WHERE expires_at IS NOT NULL;

-- Audit policy configuration (per domain)
CREATE TABLE audit_policy_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    domain_id UUID NOT NULL,
    audit_level TEXT NOT NULL DEFAULT 'level_2',
    audit_admin_actions BOOLEAN NOT NULL DEFAULT true,
    audit_security_events BOOLEAN NOT NULL DEFAULT true,
    audit_user_mail_actions BOOLEAN NOT NULL DEFAULT false,
    audit_api_calls BOOLEAN NOT NULL DEFAULT false,
    retention_days INT NOT NULL DEFAULT 90,
    mask_mail_content BOOLEAN NOT NULL DEFAULT true,
    mask_recipient_emails BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT fk_audit_policy_company FOREIGN KEY (company_id)
        REFERENCES companies(id) ON DELETE CASCADE,
    CONSTRAINT fk_audit_policy_domain FOREIGN KEY (domain_id)
        REFERENCES domains(id) ON DELETE CASCADE,
    CONSTRAINT unique_audit_policy UNIQUE (company_id, domain_id),
    CONSTRAINT valid_audit_level CHECK (audit_level IN ('level_1', 'level_2', 'level_3')),
    CONSTRAINT valid_retention CHECK (retention_days > 0)
);

CREATE INDEX idx_audit_policy_company ON audit_policy_configs(company_id);
CREATE INDEX idx_audit_policy_domain ON audit_policy_configs(domain_id);

-- Admin audit logs (admin actions + security events)
CREATE TABLE admin_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    admin_user_id UUID NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id UUID,
    changes JSONB,
    ip_address INET,
    user_agent TEXT,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_admin_audit_log_company FOREIGN KEY (company_id)
        REFERENCES companies(id) ON DELETE CASCADE,
    CONSTRAINT fk_admin_audit_log_user FOREIGN KEY (admin_user_id)
        REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_admin_audit_log_company_timestamp ON admin_audit_logs(company_id, timestamp DESC);
CREATE INDEX idx_admin_audit_log_user_timestamp ON admin_audit_logs(admin_user_id, timestamp DESC);
CREATE INDEX idx_admin_audit_log_action ON admin_audit_logs(action);
CREATE INDEX idx_admin_audit_log_resource ON admin_audit_logs(resource_type, resource_id);

-- Login audit logs (user authentication tracking)
CREATE TABLE login_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    company_id UUID NOT NULL,
    ip_address INET,
    user_agent TEXT,
    success BOOLEAN NOT NULL,
    failure_reason TEXT,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_login_audit_user FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_login_audit_company FOREIGN KEY (company_id)
        REFERENCES companies(id) ON DELETE CASCADE
);

CREATE INDEX idx_login_audit_user_timestamp ON login_audit_logs(user_id, timestamp DESC);
CREATE INDEX idx_login_audit_company_timestamp ON login_audit_logs(company_id, timestamp DESC);
CREATE INDEX idx_login_audit_success ON login_audit_logs(success, timestamp DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_login_audit_success;
DROP INDEX IF EXISTS idx_login_audit_company_timestamp;
DROP INDEX IF EXISTS idx_login_audit_user_timestamp;
DROP TABLE IF EXISTS login_audit_logs;

DROP INDEX IF EXISTS idx_admin_audit_log_resource;
DROP INDEX IF EXISTS idx_admin_audit_log_action;
DROP INDEX IF EXISTS idx_admin_audit_log_user_timestamp;
DROP INDEX IF EXISTS idx_admin_audit_log_company_timestamp;
DROP TABLE IF EXISTS admin_audit_logs;

DROP INDEX IF EXISTS idx_audit_policy_domain;
DROP INDEX IF EXISTS idx_audit_policy_company;
DROP TABLE IF EXISTS audit_policy_configs;

DROP INDEX IF EXISTS idx_user_role_expires;
DROP INDEX IF EXISTS idx_user_role_role;
DROP INDEX IF EXISTS idx_user_role_user;
DROP INDEX IF EXISTS idx_user_role_company;
DROP TABLE IF EXISTS admin_user_roles;

DROP INDEX IF EXISTS idx_permission_resource;
DROP INDEX IF EXISTS idx_permission_role;
DROP TABLE IF EXISTS admin_role_permissions;

DROP INDEX IF EXISTS idx_admin_role_builtin;
DROP INDEX IF EXISTS idx_admin_role_company;
DROP TABLE IF EXISTS admin_role_definitions;

-- +goose StatementEnd
