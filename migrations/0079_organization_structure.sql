-- +goose Up
-- +goose StatementBegin

-- organization_units: 부서/조직 단위
CREATE TABLE organization_units (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES organization_units(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    name_normalized VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL DEFAULT 'department', -- department, team, division, etc
    description TEXT,
    display_name VARCHAR(255),
    manager_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, archived, inactive
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(company_id, parent_id, name_normalized)
);

-- organization_members: 조직 내 사용자 할당
CREATE TABLE organization_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_unit_id UUID NOT NULL REFERENCES organization_units(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'member', -- member, manager, admin
    title VARCHAR(255), -- Job title
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMP WITH TIME ZONE,
    is_primary BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(organization_unit_id, user_id, started_at)
);

-- organization_sync_log: LDAP 동기화 로그
CREATE TABLE organization_sync_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    sync_source VARCHAR(50) NOT NULL DEFAULT 'ldap', -- ldap, azure_ad, okta, etc
    started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(50) NOT NULL DEFAULT 'in_progress', -- in_progress, success, failed
    units_created INT DEFAULT 0,
    units_updated INT DEFAULT 0,
    users_synced INT DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 인덱스
CREATE INDEX idx_organization_units_company_id ON organization_units(company_id);
CREATE INDEX idx_organization_units_parent_id ON organization_units(parent_id);
CREATE INDEX idx_organization_units_manager_id ON organization_units(manager_user_id);
CREATE INDEX idx_organization_members_unit_id ON organization_members(organization_unit_id);
CREATE INDEX idx_organization_members_user_id ON organization_members(user_id);
CREATE INDEX idx_organization_members_ended_at ON organization_members(ended_at) WHERE ended_at IS NOT NULL;
CREATE INDEX idx_organization_sync_log_company_id ON organization_sync_log(company_id);
CREATE INDEX idx_organization_sync_log_started_at ON organization_sync_log(started_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_organization_sync_log_started_at;
DROP INDEX IF EXISTS idx_organization_sync_log_company_id;
DROP INDEX IF EXISTS idx_organization_members_ended_at;
DROP INDEX IF EXISTS idx_organization_members_user_id;
DROP INDEX IF EXISTS idx_organization_members_unit_id;
DROP INDEX IF EXISTS idx_organization_units_manager_id;
DROP INDEX IF EXISTS idx_organization_units_parent_id;
DROP INDEX IF EXISTS idx_organization_units_company_id;

DROP TABLE IF EXISTS organization_sync_log;
DROP TABLE IF EXISTS organization_members;
DROP TABLE IF EXISTS organization_units;

-- +goose StatementEnd
