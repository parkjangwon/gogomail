# ACTIVE_TASK

## TASK-281: IMAP restored EXISTS event ordering audit

### 배경

복구된 메시지는 `EnsureIMAPMessageUIDsForMessages`로 UID/sequence를
보장한 뒤 IMAP EXISTS 이벤트를 발행한다. APPEND/COPY 경로는
`Messages=SequenceNumber`를 넣어 서버가 정확한 EXISTS count로 갱신하지만,
UID 기반 restore 이벤트는 `Messages=0`이라 서버가 단순 증가시킨다. 선택된
mailbox count가 이미 뒤처져 있거나 gap이 있으면 EXISTS 수가 낮게 나갈 수
있으므로 restore 이벤트도 sequence number를 메시지 count로 보내야 한다.

### 구현 대상

- `internal/mailservice/service.go`
- `internal/mailservice/service_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] UID 기반 EXISTS 이벤트가 `Messages=SequenceNumber`를 포함한다.
- [x] 단건/벌크/스레드 복구 이벤트 테스트가 EXISTS count를 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-282: IMAP restored EXISTS coalescing audit
