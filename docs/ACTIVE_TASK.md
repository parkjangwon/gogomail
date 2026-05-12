# ACTIVE_TASK

## TASK-228: CardDAV/CalDAV collection xml:lang If header trailing garbage audit

### 배경

TASK-227에서 non-matching tagged WebDAV `If` list의 condition-list 문법도
resource-tag relevance 전에 검증되는지 확인했다. 이제 WebDAV `If` condition
list 뒤에 남는 trailing token이 무시되어 성공하거나 412로 흐르지 않고,
HTTP 400 문법 오류로 collection `PROPPATCH` 본문을 읽기 전에 `xml:lang`
mutation을 차단하는지 확인한다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/*_test.go`
- `internal/carddavgw/handler.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV WebDAV `If` header의 condition-list 뒤 trailing token을 HTTP 400으로 거부한다.
- [x] CardDAV WebDAV `If` header의 condition-list 뒤 trailing token을 HTTP 400으로 거부한다.
- [x] CalDAV trailing-token `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] CardDAV trailing-token `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-229: CardDAV/CalDAV collection xml:lang If header malformed prefix audit
