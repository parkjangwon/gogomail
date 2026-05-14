# ACTIVE_TASK

## TASK-068: Identity Provider Abstraction

### 배경

User and group provisioning is currently database-only (hardcoded in users/groups tables).
The backend-roadmap requires a pluggable identity provider abstraction to support:
1. Database-only (default, current state)
2. LDAP/Active Directory (via bind, search, sync)
3. Azure AD (via Microsoft Graph API)
4. External RDBMS (SQL query against remote database)

Per-domain IdP configuration allows each domain to use a different identity source,
enabling organizations to federate multiple identity providers in a single deployment.

TASK-068 defines the `IdentityProvider` interface and pluggable provider registry,
allowing downstream TASK-069/070/071/072/073 to implement concrete providers.

### 구현 대상

- [x] `internal/idprovider/interface.go` — `IdentityProvider` interface (User/Group CRUD, search, sync)
- [x] `internal/idprovider/registry.go` — pluggable provider registry (Get by type, Register)
- [x] `internal/idprovider/models.go` — User, Group, Member domain models (aligned with maildb)
- [x] `internal/idprovider/database/provider.go` — Database provider implementation (wraps maildb)
- [x] Database schema update for `idp_configurations` table (per-domain IdP config)
- [x] `internal/idprovider/*_test.go` — unit tests for interface, registry, database provider
- [x] Update `maildb.User` and `maildb.Group` exports for idprovider import
- [x] `docs/ACTIVE_TASK.md`
- [x] `migrations/0102_idp_configurations.sql`

### 완료 조건

- [x] `IdentityProvider` interface defined with methods: GetUser, GetGroup, ListUsers, ListGroups, CreateUser, UpdateUser, DeleteUser, CreateGroup, DeleteGroup, AddMember, RemoveMember
- [x] `IdentityProviderRegistry` with Get(providerType string) and Register(providerType string, provider IdentityProvider)
- [x] `idprovider.User` and `idprovider.Group` domain models align with maildb.User/Group structure
- [x] Database provider implements IdentityProvider by wrapping maildb repositories
- [x] `idp_configurations` table stores per-domain IdP configuration (domain_id, provider_type, config jsonb)
- [x] Database migration creates idp_configurations table with domain_id foreign key
- [x] Unit tests for IdentityProvider interface contract (CRUD, search)
- [x] Unit tests for registry (register, get, unknown provider error)
- [x] Unit tests for database provider (wraps maildb, returns correct user/group)
- [x] `go test -count=1 ./internal/idprovider ./internal/idprovider/database -v` 통과
- [x] `go test ./...` 통과
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-069: Database Identity Mode
