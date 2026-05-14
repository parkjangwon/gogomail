# ACTIVE_TASK

## TASK-066: Organization Management

### 배경

Organization unit CRUD, hierarchy, membership, and sync handlers exist in `internal/httpapi/orgchart.go`,
and repository/service code exists in `internal/orgchart`, but the routes are not wired into the admin
runtime. The HTTP interface also uses `interface{}` for context, so the real `orgchart.Service` cannot
be used as the handler service without an adapter. This leaves the backend organization management
surface effectively unavailable in production modes.

### 구현 대상

- `internal/httpapi/orgchart.go`
- `internal/httpapi/*orgchart*_test.go`
- `internal/app/run.go`
- `internal/app/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/NEXT_STEPS.md`
- `docs/backend-roadmap.md`
- `docs/openapi.yaml` (only if the organization contract changes)

### 완료 조건

- [x] `httpapi.OrgChartService` uses `context.Context`, allowing `*orgchart.Service` to satisfy the route boundary.
- [x] admin HTTP runtime registers organization unit, hierarchy, member, and sync routes with `orgchart.NewService(orgchart.NewRepository(db), ...)`.
- [x] organization management routes remain protected by the existing admin auth token path.
- [x] focused tests verify the real service satisfies the HTTP route interface and existing organization route tests still pass.
- [x] `go test -count=1 ./internal/httpapi ./internal/app ./internal/orgchart -run 'Org|Organization'` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-067: Audit Logs (Level 1 + 2)
