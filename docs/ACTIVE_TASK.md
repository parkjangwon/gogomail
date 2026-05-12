# ACTIVE_TASK

## TASK-220: CardDAV/CalDAV collection xml:lang multi-list If header audit

### 배경

TASK-219에서 WebDAV `If` condition-list 내부의 `Not` 부정 동작을 검증했다.
RFC 4918 `If` 헤더는 여러 condition-list를 포함할 수 있고, relevant list
중 하나라도 성공하면 전체 precondition이 성공한다. collection `PROPPATCH`
경로에서 이 OR 동작이 `xml:lang` 보존과 body read 이전 실패 처리 모두에서
유지되는지 확인한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV multi-list WebDAV `If` 헤더가 첫 list 실패 후 다음 list 성공 시 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CardDAV multi-list WebDAV `If` 헤더가 첫 list 실패 후 다음 list 성공 시 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CalDAV multi-list WebDAV `If` 헤더의 모든 relevant list가 실패하면 body 적용 전에 collection language mutation을 차단한다.
- [x] CardDAV multi-list WebDAV `If` 헤더의 모든 relevant list가 실패하면 body 적용 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-221: CardDAV/CalDAV collection xml:lang If header malformed syntax audit
