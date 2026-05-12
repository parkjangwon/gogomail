# ACTIVE_TASK

## TASK-226: CardDAV/CalDAV collection xml:lang If header repeated malformed audit

### 배경

TASK-225에서 반복 HTTP `If` 헤더가 WebDAV condition-list sequence로 결합되어
성공/실패 precondition semantics를 유지하는지 확인했다. 이제 반복 헤더 중
뒤쪽 값이 malformed일 때, 앞쪽 condition-list가 이미 current ETag와
일치하더라도 전체 WebDAV `If` 문법 오류를 HTTP 400으로 거부하고 collection
`PROPPATCH` 본문을 읽기 전에 `xml:lang` mutation을 차단하는지 확인한다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/*_test.go`
- `internal/carddavgw/handler.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV repeated WebDAV `If` 헤더에서 앞쪽 matching list 뒤의 malformed list를 HTTP 400으로 거부한다.
- [x] CardDAV repeated WebDAV `If` 헤더에서 앞쪽 matching list 뒤의 malformed list를 HTTP 400으로 거부한다.
- [x] CalDAV malformed repeated WebDAV `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] CardDAV malformed repeated WebDAV `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-227: CardDAV/CalDAV collection xml:lang If header irrelevant tagged malformed audit
