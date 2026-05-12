# ACTIVE_TASK

## TASK-222: CardDAV/CalDAV collection xml:lang If header absolute URI tag audit

### 배경

TASK-221에서 malformed WebDAV `If` 헤더를 HTTP 400으로 차단함을 검증했다.
이제 RFC 4918 tagged-list의 resource tag가 absolute HTTP(S) URI인 경우에도
path가 현재 collection과 일치하면 precondition이 성공하고, path가 다르면
body read 이전에 실패하는지 확인한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV absolute URI tagged WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CardDAV absolute URI tagged WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CalDAV absolute URI tagged WebDAV `If` 헤더 path mismatch가 body 적용 전에 collection language mutation을 차단한다.
- [x] CardDAV absolute URI tagged WebDAV `If` 헤더 path mismatch가 body 적용 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-223: CardDAV/CalDAV collection xml:lang If header state-token audit
