# ACTIVE_TASK

## TASK-232: CardDAV/CalDAV collection xml:lang If header duplicate resource tag delimiter audit

### 배경

TASK-231에서 resource tag 뒤 suffix 토큰이 malformed resource tag로 HTTP 400
처리되는지 확인했다. 이제 resource tag 종료 delimiter가 중복된 값이 유효한
tag처럼 흡수되어 412로 흐르지 않고, malformed resource tag로 HTTP 400 처리되어
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

- [x] CalDAV WebDAV `If` header의 duplicate resource-tag delimiter를 HTTP 400으로 거부한다.
- [x] CardDAV WebDAV `If` header의 duplicate resource-tag delimiter를 HTTP 400으로 거부한다.
- [x] CalDAV duplicate-delimiter resource tag `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] CardDAV duplicate-delimiter resource tag `If` 헤더가 body read 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-233: POP3 multiline RETR/TOP dot-stuffing audit
