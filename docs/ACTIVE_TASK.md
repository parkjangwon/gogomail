# ACTIVE_TASK

## TASK-215: CardDAV/CalDAV collection xml:lang conditional success preservation audit

### 배경

TASK-214에서 collection precondition 실패가 body read 전에 차단되며 language
mutation을 적용하지 않음을 검증했다. 이제 collection precondition이 성공하는
`PROPPATCH` 경로에서도 omitted `xml:lang` 보존과 explicit `xml:lang` 전달이
정상 동작하는지 검증한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `internal/caldavgw/postgres_integration_test.go`
- `internal/carddavgw/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV matching `If-Match` 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CardDAV matching `If-Match` 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CalDAV `If-Match: *` 성공 PROPPATCH가 explicit `xml:lang`과 observed collection ETag를 함께 전달한다.
- [x] CardDAV `If-Match: *` 성공 PROPPATCH가 explicit `xml:lang`과 observed collection ETag를 함께 전달한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-216: CardDAV/CalDAV collection xml:lang If header conditional audit
