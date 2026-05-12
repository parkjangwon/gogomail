# ACTIVE_TASK

## TASK-230: CardDAV/CalDAV collection xml:lang If header empty resource tag audit

### 배경

TASK-229에서 WebDAV `If` condition-list 앞 prefix가 비어 있거나
`<resource-tag>` 형식이어야 함을 검증했다. 이제 `<resource-tag>` 형식처럼
보이지만 내용이 빈 `<>` prefix가 current collection precondition으로 오인되지
않고 HTTP 400으로 거부되어, collection `PROPPATCH` 본문을 읽기 전에
`xml:lang` mutation을 차단하는지 확인한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV WebDAV `If` header의 empty resource tag prefix를 HTTP 400으로 거부한다.
- [x] CardDAV WebDAV `If` header의 empty resource tag prefix를 HTTP 400으로 거부한다.
- [x] CalDAV empty-resource-tag `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] CardDAV empty-resource-tag `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-231: CardDAV/CalDAV collection xml:lang If header malformed resource tag suffix audit
