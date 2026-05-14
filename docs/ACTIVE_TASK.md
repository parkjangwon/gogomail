# ACTIVE_TASK

## TASK-070: LDAP Identity Config & Sync

### 배경

TASK-069 delivered the database identity mode as the default. TASK-070 extends the system to support LDAP as an alternative identity provider, enabling organizations to delegate user and group management to their existing directory services.

The LDAP provider will:
1. Implement the IdentityProvider interface for LDAP directory operations
2. Support per-domain LDAP configuration via ConfigRepository
3. Sync user/group data from LDAP to the local database on-demand
4. Handle LDAP connection pooling and error resilience
5. Support both simple bind and SASL authentication methods

### 구현 대상

- `internal/idprovider/ldap/provider.go` — LDAP provider implementation
- `internal/idprovider/ldap/provider_test.go` — LDAP provider tests
- `internal/idprovider/ldap/sync.go` — User/group sync logic
- `migrations/0103_ldap_sync_metadata.sql` — Schema for LDAP sync tracking
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`

### 완료 조건

- [ ] LDAP provider implements IdentityProvider interface
- [ ] ConfigRepository supports per-domain LDAP configuration storage
- [ ] User sync from LDAP to local database with conflict resolution
- [ ] Group sync from LDAP to local database
- [ ] LDAP connection pooling with timeout and retry logic
- [ ] Error handling for LDAP authentication failures
- [ ] All LDAP provider tests pass
- [ ] `go test ./...` 통과
- [ ] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-071: SAML2/OAuth2 Federation Support

---

## TASK-069: Database Identity Mode (COMPLETE)

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

- [x] Database provider fully implements IdentityProvider interface with all CRUD operations
- [x] GetUser by user ID and by email address
- [x] ListUsers with org filter, search query, limit, offset
- [x] CreateUser validates unique username per domain, required fields
- [x] UpdateUser allows status, role, settings changes
- [x] DeleteUser soft-deletes by setting status to 'deleted'
- [x] GetGroup, ListGroups with org filter
- [x] CreateGroup validates unique slug per domain
- [x] DeleteGroup soft-deletes by setting status to 'deleted'
- [x] AddMember supports user/org/group/resource membership
- [x] RemoveMember soft-deletes membership records
- [x] Per-domain IdP configuration lookup with fallback to database mode
- [x] App startup registers database provider and initializes per-domain configs
- [x] All provider tests pass with CreateUser/UpdateUser/DeleteUser covering full lifecycle
- [x] IdP configuration CRUD tests pass
- [x] `go test -count=1 ./internal/idprovider ./internal/idprovider/database ./internal/config -v` 통과
- [x] `go test ./...` 통과
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-070: LDAP Identity Config & Sync
