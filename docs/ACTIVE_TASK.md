# ACTIVE_TASK

## TASK-441: CalDAV/CardDAV inactive principal policy audit

### 배경

CalDAV/CardDAV collection creation must honor gogomail DB principal policy: users, domains, and
companies must all be active before new calendars/address books can be created. The repositories
already join active user/domain/company rows for create paths; add Postgres integration coverage so
disabled user/domain/company state cannot silently create DAV collections.

### 구현 대상

- `internal/caldavgw/postgres_integration_test.go`
- `internal/carddavgw/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `CreateCalendar`가 disabled user/domain/company 상태에서 `active user not found`로 거절되는지 검증한다.
- [x] CardDAV `CreateAddressBook`가 disabled user/domain/company 상태에서 `active user not found`로 거절되는지 검증한다.
- [x] `go test -count=1 ./internal/caldavgw -run TestPostgresCreateCalendarRejectsInactivePrincipalPolicy` 통과.
- [x] `go test -count=1 ./internal/carddavgw -run TestPostgresCreateAddressBookRejectsInactivePrincipalPolicy` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-442: CalDAV/CardDAV inactive principal read policy audit
