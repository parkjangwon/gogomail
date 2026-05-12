# ACTIVE_TASK

## TASK-216: CardDAV/CalDAV collection xml:lang If header conditional audit

### 배경

TASK-215에서 collection `If-Match` 성공 경로의 omitted/explicit `xml:lang`
동작을 검증했다. 이제 RFC 4918 WebDAV `If` 헤더가 collection `PROPPATCH`
precondition으로 쓰일 때도 동일하게 body 적용 전 조건을 검증하고,
성공 경로에서 language tag 보존/전달이 깨지지 않는지 확인한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV matching WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CardDAV matching WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CalDAV failing WebDAV `If` 헤더 PROPPATCH가 body 적용 전에 collection language mutation을 차단한다.
- [x] CardDAV failing WebDAV `If` 헤더 PROPPATCH가 body 적용 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-217: CardDAV/CalDAV collection xml:lang If header PostgreSQL integration audit
