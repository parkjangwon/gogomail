# ACTIVE_TASK

## TASK-221: CardDAV/CalDAV collection xml:lang If header malformed syntax audit

### 배경

TASK-220에서 WebDAV `If` 헤더의 multi-list OR 동작을 검증했다. 이제
collection `PROPPATCH`에서 malformed `If` 헤더가 HTTP 400으로 실패하고,
XML body를 읽거나 `xml:lang` mutation을 적용하기 전에 차단되는지 확인한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV malformed WebDAV `If` 헤더 PROPPATCH가 400으로 실패하고 body 적용 전에 collection language mutation을 차단한다.
- [x] CardDAV malformed WebDAV `If` 헤더 PROPPATCH가 400으로 실패하고 body 적용 전에 collection language mutation을 차단한다.
- [x] CalDAV/CardDAV 테스트가 unsupported condition과 unterminated entity-tag 오류를 모두 포함한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-222: CardDAV/CalDAV collection xml:lang If header absolute URI tag audit
