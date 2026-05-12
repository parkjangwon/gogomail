# ACTIVE_TASK

## TASK-211: CardDAV/CalDAV collection xml:lang repository nil-preservation audit

### 배경

TASK-209와 TASK-210에서 migrated PostgreSQL schema의 language round-trip과
DB-level invalid value rejection은 검증되었다. 이제 repository update 요청에서
`xml:lang`이 생략된 경우 기존 `displayname_lang`/`description_lang` 값이
의도치 않게 지워지지 않는지 검증한다.

### 구현 대상

- `internal/caldavgw/xml.go`
- `internal/caldavgw/repository.go`
- `internal/caldavgw/*_test.go`
- `internal/carddavgw/xml.go`
- `internal/carddavgw/repository.go`
- `internal/carddavgw/*_test.go`
- `internal/caldavgw/postgres_integration_test.go`
- `internal/carddavgw/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV PostgreSQL integration test가 unrelated property update에서 기존 language columns를 보존한다.
- [x] CalDAV PostgreSQL integration test가 displayname/description text-only update에서 기존 language columns를 보존한다.
- [x] CardDAV PostgreSQL integration test가 displayname/description text-only update에서 기존 language columns를 보존한다.
- [x] CalDAV/CardDAV PROPPATCH parser가 absent `xml:lang`과 explicit empty `xml:lang`을 구분한다.
- [x] CalDAV/CardDAV repository update validation이 omitted language pointer를 nil로 보존한다.
- [x] CalDAV/CardDAV handler tests가 `xml:lang` 없는 text update에서 기존 language tag 보존을 검증한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-212: CardDAV/CalDAV collection xml:lang empty-value clearing audit
