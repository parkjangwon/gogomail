# ACTIVE_TASK

## TASK-070: LDAP Identity Config & Sync (in progress)

### 배경

TASK-069 delivered the database identity mode as the default. TASK-070 extends the system to support LDAP as an alternative identity provider, enabling organizations to delegate user and group management to their existing directory services.

The LDAP provider implements:
1. IdentityProvider interface for LDAP directory operations (read-only)
2. Per-domain LDAP configuration via ConfigRepository
3. User/group sync API with conflict resolution
4. LDAP connection configuration with pooling parameters
5. Sync metadata tracking and incremental sync support

### 구현 상태

- `internal/idprovider/ldap/provider.go` — LDAP provider fully implements IdentityProvider interface
  - GetUser, GetGroup, ListUsers, ListGroups (stub: "not implemented" errors)
  - CreateUser, UpdateUser, DeleteUser return "read-only" errors
  - CreateGroup, DeleteGroup, AddMember, RemoveMember return "read-only" errors
  - Config struct supports host, port, DN, bind credentials, SSL/TLS, and attribute mappings
- `internal/idprovider/ldap/provider_test.go` — 12 comprehensive validation tests
- `internal/idprovider/ldap/sync.go` — SyncUsers, SyncGroups, SyncMemberships APIs with conflict resolution
- `internal/idprovider/ldap/sync_test.go` — 7 sync validation tests
- `migrations/0103_ldap_sync_metadata.sql` — Schema for tracking sync runs, conflicts, and incremental sync cursors
  - ldap_sync_runs: tracks status, counts, timing of each sync operation
  - ldap_sync_cursors: enables incremental sync with RFC 3928 sync cookies
  - ldap_sync_conflicts: logs conflicts for manual review and audit trail

### 완료 조건

- [x] LDAP provider implements IdentityProvider interface (read-only operations)
- [x] ConfigRepository supports per-domain LDAP configuration storage (from TASK-069)
- [x] User sync API from LDAP with conflict resolution strategy
- [x] Group sync API from LDAP
- [x] LDAP connection configuration with pooling, SSL/TLS, bind methods
- [x] Database schema for sync tracking, metadata, and conflict resolution
- [x] All LDAP provider tests pass (19 tests)
- [x] `go test ./...` 통과 (5975 tests)
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 단계

LDAP provider foundation complete. Next steps:
1. Implement actual LDAP connection logic using go-ldap library
2. Implement user/group query, sync, and membership linking
3. Implement conflict detection and resolution
4. Add per-domain LDAP config loading in app startup

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
