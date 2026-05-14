# ACTIVE_TASK

## TASK-064: Admin Auth & Session — JWT, login, refresh

### 배경

Admin auth routes have a login handler and JWT middleware hooks, but session behavior is still weak:
`/auth/verify` returns authenticated without validating a token, logout is client-only, there is no
refresh endpoint, and admin-only deployments do not reliably initialize the JWT token manager. The
backend needs a real admin session boundary before frontend implementation starts.

### 구현 대상

- `internal/httpapi/admin.go`
- `internal/httpapi/*_test.go`
- `internal/app/run.go`
- `internal/app/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/NEXT_STEPS.md`
- `docs/backend-roadmap.md`
- `docs/openapi.yaml`

### 완료 조건

- [x] admin login issues signed access and refresh tokens when `GOGOMAIL_AUTH_JWT_SECRET` is configured.
- [x] admin-only HTTP modes initialize the JWT token manager and session-version checker.
- [x] `/admin/v1/auth/verify` validates bearer JWTs with session-version revocation instead of returning unconditional success.
- [x] `/admin/v1/auth/refresh` accepts a valid refresh token and issues a new access token with the same admin claims.
- [x] `/admin/v1/auth/logout` invalidates server-side sessions by incrementing `session_version` when a signed bearer token is provided.
- [x] focused HTTP/app tests cover login, verify rejection/success, refresh, logout revocation, and admin-mode token-manager wiring.
- [x] `go test -count=1 ./internal/httpapi ./internal/app ./internal/auth -run 'Admin.*Auth|Admin.*Session|Token|JWT|Refresh|Logout|Verify'` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-065: User Management CRUD
