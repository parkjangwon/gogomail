# ACTIVE_TASK

## TASK-219: CardDAV/CalDAV collection xml:lang Not If header audit

### 배경

TASK-216~218에서 WebDAV `If` 헤더의 untagged/tagged entity-tag
조건을 collection `PROPPATCH` 경로에서 검증했다. RFC 4918 `If` 조건은
`Not` 부정을 허용하므로, 부정 조건 성공과 실패 모두에서 body read 이전
precondition 평가 및 omitted `xml:lang` 보존 동작을 확인한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV `Not` WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CardDAV `Not` WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CalDAV `Not` WebDAV `If` 헤더 실패 PROPPATCH가 body 적용 전에 collection language mutation을 차단한다.
- [x] CardDAV `Not` WebDAV `If` 헤더 실패 PROPPATCH가 body 적용 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-220: CardDAV/CalDAV collection xml:lang multi-list If header audit
