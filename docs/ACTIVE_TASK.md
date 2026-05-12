# ACTIVE_TASK

## TASK-229: CardDAV/CalDAV collection xml:lang If header malformed prefix audit

### 배경

TASK-228에서 WebDAV `If` condition-list 뒤 trailing token이 HTTP 400으로
거부되는지 확인했다. 이제 condition-list 앞 prefix가 비어 있거나
`<resource-tag>` 형식이어야 한다는 문법을 검증해, 알 수 없는 prefix가
untagged list로 오인되어 성공하지 않고 collection `PROPPATCH` 본문을 읽기 전에
`xml:lang` mutation을 차단하는지 확인한다.

### 구현 대상

- `internal/caldavgw/handler.go`
- `internal/caldavgw/*_test.go`
- `internal/carddavgw/handler.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV WebDAV `If` header의 malformed condition-list prefix를 HTTP 400으로 거부한다.
- [x] CardDAV WebDAV `If` header의 malformed condition-list prefix를 HTTP 400으로 거부한다.
- [x] CalDAV malformed-prefix `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] CardDAV malformed-prefix `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-230: CardDAV/CalDAV collection xml:lang If header empty resource tag audit
