# ACTIVE_TASK

## TASK-210: CardDAV/CalDAV collection xml:lang invalid postgres constraint audit

### 배경

TASK-209에서 migrated PostgreSQL schema의 CalDAV/CardDAV repository
create/get/update language round-trip은 검증되었다. 이제 application validation을
우회한 raw SQL mutation도 migration 0097의 DB-level language 제약에 의해
차단되는지 검증한다.

### 구현 대상

- `internal/caldavgw/postgres_integration_test.go`
- `internal/carddavgw/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV PostgreSQL integration test가 whitespace/control `displayname_lang` mutation을 DB 제약으로 거부한다.
- [x] CalDAV PostgreSQL integration test가 과도하게 긴 `description_lang` mutation을 DB 제약으로 거부한다.
- [x] CardDAV PostgreSQL integration test가 whitespace/control `displayname_lang` mutation을 DB 제약으로 거부한다.
- [x] CardDAV PostgreSQL integration test가 과도하게 긴 `description_lang` mutation을 DB 제약으로 거부한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-211: CardDAV/CalDAV collection xml:lang repository nil-preservation audit
