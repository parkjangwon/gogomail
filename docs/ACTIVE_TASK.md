# ACTIVE_TASK

## TASK-227: CardDAV/CalDAV collection xml:lang If header irrelevant tagged malformed audit

### 배경

TASK-226에서 반복 HTTP `If` 헤더의 뒤쪽 malformed relevant condition-list가
앞쪽 matching list 뒤에 있어도 HTTP 400으로 거부되는지 확인했다. 이제 tagged
WebDAV `If` list의 resource tag가 현재 collection과 일치하지 않더라도,
condition-list 문법 자체는 검증되어 malformed 입력이 HTTP 400으로 거부되고
collection `PROPPATCH` 본문을 읽기 전에 `xml:lang` mutation을 차단하는지
확인한다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/*_test.go`
- `internal/carddavgw/handler.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV non-matching tagged WebDAV `If` list의 malformed condition-list를 HTTP 400으로 거부한다.
- [x] CardDAV non-matching tagged WebDAV `If` list의 malformed condition-list를 HTTP 400으로 거부한다.
- [x] CalDAV malformed non-matching tagged `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] CardDAV malformed non-matching tagged `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-228: CardDAV/CalDAV collection xml:lang If header trailing garbage audit
