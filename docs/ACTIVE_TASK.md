# ACTIVE_TASK

## TASK-224: CardDAV/CalDAV collection xml:lang If header compound condition audit

### 배경

TASK-223에서 WebDAV `If` state-token 조건의 no-lock-store 동작을 검증했다.
이제 한 condition-list 안에 여러 조건이 들어가는 compound condition에서
모든 조건이 AND로 평가되는지, collection `PROPPATCH`의 `xml:lang` 보존과
body read 이전 실패 처리 모두에서 확인한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV compound WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CardDAV compound WebDAV `If` 헤더 성공 PROPPATCH가 omitted `xml:lang`에서 기존 language tag를 보존한다.
- [x] CalDAV compound WebDAV `If` 헤더의 일부 조건 실패가 body 적용 전에 collection language mutation을 차단한다.
- [x] CardDAV compound WebDAV `If` 헤더의 일부 조건 실패가 body 적용 전에 collection language mutation을 차단한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-225: CardDAV/CalDAV collection xml:lang If header repeated header audit
