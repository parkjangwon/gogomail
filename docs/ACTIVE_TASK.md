# ACTIVE_TASK

## TASK-063: Admin Console Schema + RBAC + Custom Roles

### 배경

The admin console schema migration already creates role, permission, and user-role tables, but the
Admin API role endpoints still return hard-coded mock role data and do not persist custom role
creation. Backend operators need the role list/create contract backed by the real RBAC schema before
any frontend work starts.

### 구현 대상

- `internal/admin/*`
- `internal/app/admin_service.go`
- `internal/httpapi/admin.go`
- `internal/httpapi/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`
- `docs/NEXT_STEPS.md`
- `docs/openapi.yaml` (only if the role contract changes)

### 완료 조건

- [x] `/admin/v1/roles` lists persisted company roles instead of mock data.
- [x] `POST /admin/v1/roles` validates input and persists a custom non-builtin role.
- [x] role responses expose deterministic `permissions_count` and `assigned_users` values from the RBAC tables.
- [x] role list/create paths are wired through the app admin service without starting frontend implementation.
- [x] focused HTTP/API tests cover listing roles, creating a role, validation failure, and service error mapping.
- [x] `go test -count=1 ./internal/httpapi ./internal/admin ./internal/app -run 'Role|Admin'` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-064: Admin Auth & Session — JWT, login, refresh
