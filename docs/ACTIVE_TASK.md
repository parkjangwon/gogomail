# ACTIVE_TASK

## TASK-443: DAV auth repository active policy audit

### 배경

CalDAV/CardDAV Basic auth delegates credential validation to `maildb.Repository.AuthenticatePlain`.
That repository query already requires active users, domains, and companies, but Postgres coverage
only asserted suspended company rejection. Add user/domain coverage so DAV authentication cannot
accidentally accept disabled/suspended principals if the query changes.

### 구현 대상

- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `AuthenticatePlain`이 active user/domain 상태에서는 정상 인증되는지 검증한다.
- [x] user status가 `suspended`가 되면 같은 credential 인증이 거절되는지 검증한다.
- [x] domain status가 `suspended`가 되면 같은 credential 인증이 거절되는지 검증한다.
- [x] `go test -count=1 ./internal/maildb -run TestPostgresAuthenticatePlainRejectsSuspendedUserAndDomain` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-444: DAV auth repository must-change-password policy audit
