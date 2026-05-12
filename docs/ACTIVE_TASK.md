# ACTIVE_TASK

## TASK-213: CardDAV/CalDAV collection xml:lang unsupported-property rollback audit

### 배경

TASK-211과 TASK-212에서 omitted `xml:lang` 보존과 explicit empty clearing은
검증되었다. 이제 RFC 4918 PROPPATCH atomicity에 따라 unsupported/protected
property가 섞인 실패 요청에서 text mutation뿐 아니라 language tag mutation도
함께 롤백되는지 검증한다.

### 구현 대상

- `internal/caldavgw/*_test.go`
- `internal/carddavgw/*_test.go`
- `internal/caldavgw/postgres_integration_test.go`
- `internal/carddavgw/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CalDAV unsupported-property PROPPATCH 실패가 displayname language mutation을 롤백한다.
- [x] CalDAV duplicate dependency/protected remove 실패가 description language mutation을 롤백한다.
- [x] CardDAV unsupported-property PROPPATCH 실패가 displayname language mutation을 롤백한다.
- [x] CardDAV duplicate dependency/protected remove 실패가 description language mutation을 롤백한다.
- [x] 실패 응답에서 CalDAV/CardDAV repository update가 호출되지 않음을 검증한다.
- [x] `go test ./internal/caldavgw ./internal/carddavgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-214: CardDAV/CalDAV collection xml:lang conditional failure rollback audit
