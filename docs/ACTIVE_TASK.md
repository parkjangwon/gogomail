# ACTIVE_TASK

## TASK-208: CardDAV/CalDAV collection xml:lang migration integration audit

### 배경

TASK-206과 TASK-207에서 CalDAV/CardDAV collection property의 `xml:lang`이
`PROPPATCH`, CalDAV `MKCALENDAR`, CardDAV extended `MKCOL`, collection
`PROPFIND` 흐름에 연결되었다. 이제 migration 0097이 repository SQL에서 사용하는
language column과 DB-level 제약, rollback cleanup을 계속 보장하도록 정적 회귀
테스트를 추가한다.

### 구현 대상

- `internal/database/migration_files_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] migration 0097이 CalDAV `displayname_lang`/`description_lang` columns와 제약을 선언한다.
- [x] migration 0097이 CardDAV `displayname_lang`/`description_lang` columns와 제약을 선언한다.
- [x] migration 0097 Down 경로가 CardDAV/CalDAV language 제약과 columns를 제거한다.
- [x] 정적 migration 테스트가 repository SQL이 의존하는 language column 이름을 보호한다.
- [x] `go test ./internal/database ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-209: CardDAV/CalDAV collection xml:lang postgres repository integration audit
