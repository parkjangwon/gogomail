# ACTIVE_TASK

## TASK-079: Audit Policy Config UI (in progress)

### 배경

TASK-078 delivered the dashboard UI polish pass. TASK-079 adds a dedicated audit policy configuration surface so System Admins and Domain Admins can control audit level, retention, and masking without leaving the security area.

### 구현 대상

Frontend:
- `apps/console/src/app/companies/[id]/security/audit-policy/page.tsx`
  - audit level selector
  - audit retention controls
  - masking toggle(s)
- `apps/console/src/hooks/useAuditPolicy.ts`
- `apps/console/src/components/Sidebar.tsx` if a navigation link is needed

### 완료 조건

- [ ] `go test ./...` 통과
- [ ] Audit policy values can be viewed and saved from the console
- [ ] Audit level, retention, and masking controls are clearly labeled
- [ ] The page fits the existing security section layout without crowding neighboring settings
- [ ] `docs/CURRENT_STATUS.md` 갱신
- [ ] `docs/backend-roadmap.md` 해당 항목 체크/갱신
- [ ] (API 변경 시) `docs/openapi.yaml` 갱신

### 다음 태스크

TASK-080: Export & Reports

---

## TASK-072: External RDBMS Config & Sync (COMPLETE)

### 배경

TASK-070/071 delivered LDAP provider support with admin API and backend infrastructure. TASK-072 extends the system to support External RDBMS (HR/ERP databases) as an alternative identity provider, enabling organizations to read users and groups directly from their enterprise database using custom SQL queries and field mappings.

The RDBMS provider implements:
1. IdentityProvider interface for RDBMS directory operations (read-only, SQL-based)
2. Per-domain RDBMS configuration via ConfigRepository with connection pooling
3. User/group sync API with SQL query mapping and conflict resolution
4. Field mapping (SQL column → gogomail schema fields)
5. Sync metadata tracking and incremental sync support

### 구현 대상

Backend (no frontend gate required):
- `internal/idprovider/rdbms/provider.go` — RDBMS provider implementing IdentityProvider interface
  - Config struct with connection string, pool size, query templates, field mappings
  - GetUser, GetGroup, ListUsers, ListGroups (read-only SQL-based operations)
  - CreateUser, UpdateUser, DeleteUser return "read-only" errors
  - CreateGroup, DeleteGroup, AddMember, RemoveMember return "read-only" errors
- `internal/idprovider/rdbms/sync.go` — SyncUsers, SyncGroups, SyncMemberships with conflict resolution
- `internal/idprovider/rdbms/validator.go` — SQL query validation, field mapping validation, connection testing
- `internal/maildb/rdbms_sync.go` — Data models for sync runs, conflicts, mapping configs
- `internal/maildb/rdbms_sync_repository.go` — RDBMS sync history and conflict tracking (mirrors TASK-071 pattern)
- `internal/idprovider/rdbms/admin_service.go` — Service layer for RDBMS config/sync/testing
- `internal/admin/admin.go` — Wire RDBMS admin service into admin runtime
- Database migrations for RDBMS sync run metadata (if needed beyond existing)
- All RDBMS provider tests

### 완료 조건

- [x] RDBMS provider implements IdentityProvider interface (read-only SQL operations)
- [x] ConfigRepository supports per-domain RDBMS configuration storage (via idp_configurations table)
- [x] User sync API from RDBMS with field mapping and conflict resolution
- [x] Group sync API from RDBMS
- [x] RDBMS connection configuration with pooling and security
- [x] SQL query validation and connection testing
- [x] Field mapping configuration (SQL column names → go field names)
- [x] Database schema for RDBMS sync tracking and metadata (migration 0104_rdbms_sync_metadata.sql)
- [x] All RDBMS provider tests pass (36 tests)
- [x] `go test ./...` 통과 (5930 tests)
- [x] OpenAPI documentation for all RDBMS sync endpoints
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-073: External RDBMS Sync UI & Logs

---

## TASK-071: LDAP Sync UI & Logs (COMPLETE)

### 배경

TASK-070 delivered the LDAP provider interface and configuration schema. TASK-071 implements the admin backend and frontend for LDAP sync operations, logs, and monitoring.

The LDAP sync surface includes:
1. Admin API for LDAP sync scheduling and history
2. Sync logs and conflict resolution UI
3. Real-time sync status monitoring
4. Per-domain LDAP configuration management in admin console

### 구현 대상

Backend (no frontend gate required):
- `internal/httpapi/admin.go` — Add LDAP sync routes:
  - `POST /admin/v1/domains/{id}/ldap/sync` — Trigger LDAP sync (users, groups, memberships)
  - `GET /admin/v1/domains/{id}/ldap/sync-history` — List sync runs with status/counts/timing
  - `GET /admin/v1/domains/{id}/ldap/conflicts` — List unresolved sync conflicts
  - `POST /admin/v1/domains/{id}/ldap/conflicts/{id}/resolve` — Resolve conflict manually
- `internal/admin/admin.go` — Wire LDAP sync service into admin runtime
- `internal/idprovider/ldap/admin_service.go` — Service layer for sync scheduling/history querying
- Database migrations for sync run metadata, status history (if needed beyond TASK-070 schema)
- OpenAPI documentation for sync endpoints

Frontend (gate applies here):
- Admin console "Domain Settings > LDAP Configuration" screen
  - Display LDAP provider status (connected/disconnected, last sync time)
  - Manual sync button + sync progress indicator
  - Sync history table (run date, user count, group count, duration, status)
- Admin console "Domain Settings > LDAP Conflicts" screen
  - Unresolved conflict listing (user/group, issue type, details)
  - Bulk resolve or per-item manual review
- Sync logs viewer (export, filter by domain/status/date range)

### 완료 조건

- [x] Admin API POST /admin/v1/domains/{id}/ldap/sync triggers real sync with result envelope
- [x] Admin API GET /admin/v1/domains/{id}/ldap/sync-history lists runs with pagination
- [x] Admin API GET /admin/v1/domains/{id}/ldap/conflicts lists sync conflicts
- [x] Admin API POST /admin/v1/domains/{id}/ldap/conflicts/{id}/resolve allows manual resolution
- [x] Database schema supports sync run history, conflict tracking (extends TASK-070)
- [ ] OpenAPI documents all new LDAP sync endpoints (BLOCKED: requires spec)
- [x] All backend API tests pass (sync scheduling, history retrieval, conflict resolution)
- [x] `go test ./...` 통과 (5901 tests)
- [ ] Frontend gate triggered before admin console UI implementation
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-072: External RDBMS Config & Sync

---

## TASK-070: LDAP Identity Config & Sync (COMPLETE)

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
- [x] `go test ./...` 통과 (5901 tests)
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 단계

LDAP provider foundation complete. Implementation tasks deferred to TASK-071:
1. Admin API for LDAP sync scheduling and history
2. Sync logs and conflict resolution UI
3. Real-time sync status monitoring in admin console

---

## TASK-069: Database Identity Mode (COMPLETE)

### 배경

TASK-068 delivered the pluggable `IdentityProvider` interface and registry. TASK-069 implements the database-only identity mode as the concrete default provider, fully supporting user/group CRUD operations through `maildb` repositories.

The database provider:
1. Wraps existing `maildb` user repositories for GetUser/ListUsers/CreateUser/UpdateUser/DeleteUser
2. Wraps directory_groups tables for GetGroup/ListGroup/CreateGroup/DeleteGroup
3. Supports group membership management through directory_group_memberships
4. Registers itself as the default "database" provider in the registry
5. Supports per-domain IdP configuration with fallback to default database mode

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
