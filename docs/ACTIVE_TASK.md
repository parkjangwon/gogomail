# ACTIVE_TASK

## TASK-231: CardDAV/CalDAV collection xml:lang If header malformed resource tag suffix audit

### 배경

TASK-230에서 빈 `<>` resource tag prefix가 HTTP 400으로 거부되는지 확인했다.
이제 resource tag 뒤에 붙은 suffix성 토큰이 마지막 `>` 때문에 태그 내용으로
흡수되어 412로 흐르지 않고, malformed resource tag로 HTTP 400 처리되어
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

- [x] CalDAV WebDAV `If` header의 malformed resource-tag suffix를 HTTP 400으로 거부한다.
- [x] CardDAV WebDAV `If` header의 malformed resource-tag suffix를 HTTP 400으로 거부한다.
- [x] CalDAV malformed-suffix resource tag `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] CardDAV malformed-suffix resource tag `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-232: CardDAV/CalDAV collection xml:lang If header duplicate resource tag delimiter audit
