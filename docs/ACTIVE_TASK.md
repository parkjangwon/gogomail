# ACTIVE_TASK

## TASK-444: DAV auth repository must-change-password policy audit

### 배경

CalDAV/CardDAV Basic auth resolvers reject users with `MustChangePassword`, but that depends on
`maildb.Repository.AuthenticatePlain` returning the DB `must_change_password` flag with the
authenticated user. Add Postgres coverage so the repository credential path keeps surfacing this
policy bit to DAV auth.

### 구현 대상

- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] DB user의 `must_change_password` 값을 true로 설정한 뒤 `AuthenticatePlain` 인증 경로를 검증한다.
- [x] 인증 결과가 `MustChangePassword=true`를 반환하는지 검증한다.
- [x] 인증 결과가 같은 user/domain ID를 반환해 DAV resolver가 정확한 principal 정책을 적용할 수 있는지 검증한다.
- [x] `go test -count=1 ./internal/maildb -run TestPostgresAuthenticatePlainReturnsMustChangePassword` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-445: SMTP submission must-change-password policy audit
