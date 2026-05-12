# ACTIVE_TASK

## TASK-218: CardDAV/CalDAV collection xml:lang tagged If header audit

### 배경

TASK-216에서 untagged WebDAV `If` 헤더 collection `PROPPATCH` 경로를
검증했다. RFC 4918 `If` 헤더는 tagged-list 형식(`<href> ([etag])`)도
허용하므로, collection path에 정확히 매칭되는 tagged 조건과 매칭되지 않는
tagged 조건 모두에서 `xml:lang` 보존 및 pre-body rejection 동작을 확인한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV matching tagged WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CardDAV matching tagged WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CalDAV non-matching tagged WebDAV `If` 헤더 PROPPATCH가 body 적용 전에 collection language mutation을 차단한다.
- [x] CardDAV non-matching tagged WebDAV `If` 헤더 PROPPATCH가 body 적용 전에 collection language mutation을 차단한다.
- [x] CalDAV matching tagged WebDAV `If` 헤더가 stale ETag이면 body 적용 전에 collection language mutation을 차단한다.
- [x] CardDAV matching tagged WebDAV `If` 헤더가 stale ETag이면 body 적용 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-219: CardDAV/CalDAV collection xml:lang Not If header audit
