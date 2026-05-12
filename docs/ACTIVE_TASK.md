# ACTIVE_TASK

## TASK-214: CardDAV/CalDAV collection xml:lang conditional failure rollback audit

### 배경

TASK-213에서 unsupported/protected property 실패 시 language mutation이
롤백됨을 검증했다. 이제 collection precondition 실패(`If-Match`,
`If-None-Match`, `If-Unmodified-Since`)가 PROPPATCH body를 읽기 전에 차단되며,
그 안의 `xml:lang` mutation도 적용되지 않는지 검증한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `internal/caldavgw/postgres_integration_test.go`
- `internal/carddavgw/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV conditional failure PROPPATCH가 body의 displayname language mutation을 적용하지 않는다.
- [x] CardDAV conditional failure PROPPATCH가 body의 displayname language mutation을 적용하지 않는다.
- [x] CalDAV/CardDAV conditional failure가 body read와 repository update 전에 차단됨을 검증한다.
- [x] CalDAV/CardDAV repeated `If-Unmodified-Since` 실패도 body read 전에 차단됨을 검증한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-215: CardDAV/CalDAV collection xml:lang conditional success preservation audit
