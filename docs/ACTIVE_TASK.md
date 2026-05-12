# ACTIVE_TASK

## TASK-225: CardDAV/CalDAV collection xml:lang If header repeated header audit

### 배경

TASK-224에서 한 WebDAV `If` condition-list 내부의 compound AND 동작을
검증했다. 이제 HTTP 요청에 `If` 헤더가 반복되어 들어올 때 gateway가 값들을
공백으로 결합한 뒤 동일한 precondition semantics를 적용하는지, collection
`PROPPATCH`의 `xml:lang` 보존과 body read 이전 실패 처리 모두에서 확인한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV repeated WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CardDAV repeated WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CalDAV repeated WebDAV `If` 헤더 실패가 body 적용 전에 collection language mutation을 차단한다.
- [x] CardDAV repeated WebDAV `If` 헤더 실패가 body 적용 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-226: CardDAV/CalDAV collection xml:lang If header repeated malformed audit
