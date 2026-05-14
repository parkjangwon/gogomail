# ACTIVE_TASK

## TASK-069: Database Identity Mode

### 배경

TASK-068 delivered the pluggable `IdentityProvider` interface and registry. TASK-069 implements
the database-only identity mode as the concrete default provider, fully supporting user/group
CRUD operations through `maildb` repositories.

The database provider will:
1. Wrap existing `maildb` user repositories for GetUser/ListUsers/CreateUser/UpdateUser/DeleteUser
2. Wrap directory_groups tables for GetGroup/ListGroup/CreateGroup/DeleteGroup
3. Support group membership management through directory_group_memberships
4. Register itself as the default "database" provider in the registry
5. Support per-domain IdP configuration with fallback to default database mode

### 구현 대상

- `internal/idprovider/database/repository.go` — IdP-focused repository methods (get by email, search by org)
- `internal/idprovider/database/provider.go` — Complete provider implementation (overwrite with full CRUD support)
- `internal/idprovider/database/provider_test.go` — Comprehensive tests for all CRUD operations
- `internal/idprovider/config.go` — Per-domain IdP configuration loader/validator
- `internal/app/idprovider_init.go` — App startup wiring (register database provider, load per-domain configs)
- `internal/maildb/idprovider_adapter.go` — Exports for idprovider import (User, Group models)
- Database model exports and idprovider domain model alignment
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`

### 완료 조건

- [ ] Database provider fully implements IdentityProvider interface with all CRUD operations
- [ ] GetUser by user ID and by email address
- [ ] ListUsers with org filter, search query, limit, offset
- [ ] CreateUser validates unique username per domain, required fields
- [ ] UpdateUser allows status, role, settings changes
- [ ] DeleteUser soft-deletes by setting status to 'deleted'
- [ ] GetGroup, ListGroups with org filter
- [ ] CreateGroup validates unique slug per domain
- [ ] DeleteGroup soft-deletes by setting status to 'deleted'
- [ ] AddMember supports user/org/group/resource membership
- [ ] RemoveMember soft-deletes membership records
- [ ] Per-domain IdP configuration lookup with fallback to database mode
- [ ] App startup registers database provider and initializes per-domain configs
- [ ] All provider tests pass with CreateUser/UpdateUser/DeleteUser covering full lifecycle
- [ ] IdP configuration CRUD tests pass
- [ ] `go test -count=1 ./internal/idprovider ./internal/idprovider/database ./internal/config -v` 통과
- [ ] `go test ./...` 통과
- [ ] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-070: LDAP Identity Config & Sync
