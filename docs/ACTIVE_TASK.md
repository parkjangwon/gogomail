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

- `internal/idprovider/interface.go` — `IdentityProvider` interface (User/Group CRUD, search, sync)
- `internal/idprovider/registry.go` — pluggable provider registry (Get by type, Register)
- `internal/idprovider/models.go` — User, Group, Member domain models (aligned with maildb)
- `internal/idprovider/database/provider.go` — Database provider implementation (wraps maildb)
- Database schema update for `idp_configurations` table (per-domain IdP config)
- `internal/idprovider/*_test.go` — unit tests for interface, registry, database provider
- Update `maildb.User` and `maildb.Group` exports for idprovider import
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/NEXT_STEPS.md`

### 완료 조건

- [ ] `IdentityProvider` interface defined with methods: GetUser, GetGroup, ListUsers, ListGroups, CreateUser, UpdateUser, DeleteUser, CreateGroup, DeleteGroup, AddMember, RemoveMember
- [ ] `IdentityProviderRegistry` with Get(providerType string) and Register(providerType string, provider IdentityProvider)
- [ ] `idprovider.User` and `idprovider.Group` domain models align with maildb.User/Group structure
- [ ] Database provider implements IdentityProvider by wrapping maildb repositories
- [ ] `idp_configurations` table stores per-domain IdP configuration (domain_id, provider_type, config jsonb)
- [ ] Database migration creates idp_configurations table with domain_id foreign key
- [ ] Admin can query current IdP configuration for a domain via repository method
- [ ] Unit tests for IdentityProvider interface contract (CRUD, search)
- [ ] Unit tests for registry (register, get, unknown provider error)
- [ ] Unit tests for database provider (wraps maildb, returns correct user/group)
- [ ] `go test -count=1 ./internal/idprovider ./internal/idprovider/database -v` 통과
- [ ] `go test ./...` 통과
- [ ] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-069: Database Identity Mode
