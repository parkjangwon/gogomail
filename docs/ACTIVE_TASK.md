# ACTIVE_TASK

## TASK-223: CardDAV/CalDAV collection xml:lang If header state-token audit

### 배경

TASK-222에서 absolute URI tagged-list path matching을 검증했다. 현재
CalDAV/CardDAV 서버는 WebDAV lock-token store를 구현하지 않으므로 `If`
condition-list의 state-token 조건은 false로 평가되고, `Not <token>`은 true로
평가된다. collection `PROPPATCH`에서 이 lock-token 없는 동작이 body read
이전 precondition 처리와 `xml:lang` 보존/차단을 유지하는지 확인한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV bare state-token WebDAV `If` 헤더 PROPPATCH가 body 적용 전에 collection language mutation을 차단한다.
- [x] CardDAV bare state-token WebDAV `If` 헤더 PROPPATCH가 body 적용 전에 collection language mutation을 차단한다.
- [x] CalDAV `Not` state-token WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CardDAV `Not` state-token WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-224: CardDAV/CalDAV collection xml:lang If header compound condition audit
