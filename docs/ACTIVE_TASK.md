# ACTIVE_TASK

## TASK-065: User Management CRUD

### 배경

Admin user management already supports list/get/create plus patch-style status, quota, password,
role, and recovery-email updates. The remaining CRUD gap is delete: the backend has no
`DELETE /admin/v1/users/{id}` route or repository boundary. Because the user status constraint only
allows `active`, `suspended`, and `disabled`, deletion should be a safe admin disable operation that
also revokes existing sessions.

### 구현 대상

- `internal/maildb/admin.go`
- `internal/httpapi/admin.go`
- `internal/httpapi/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/NEXT_STEPS.md`
- `docs/backend-roadmap.md`
- `docs/openapi.yaml`

### 완료 조건

- [x] `maildb.Repository` exposes a delete-user boundary that marks the user `disabled` and increments `session_version`.
- [x] `DELETE /admin/v1/users/{id}` validates the path id, rejects request bodies/query parameters, and dispatches through `AdminService`.
- [x] delete responses use the existing status envelope and surface not-found/validation failures as API errors.
- [x] OpenAPI documents `DELETE /admin/v1/users/{id}`.
- [x] focused tests cover successful delete dispatch, unsafe path rejection, and repository delete-user behavior.
- [x] `go test -count=1 ./internal/httpapi ./internal/maildb -run 'User|Delete'` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-066: Organization Management
