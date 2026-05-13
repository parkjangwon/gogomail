# ACTIVE_TASK

## TASK-282: IMAP restored EXISTS coalescing audit

### 배경

복구된 메시지는 `EnsureIMAPMessageUIDsForMessages`로 UID/sequence를 보장한 뒤
IMAP EXISTS 이벤트를 발행한다. TASK-281에서 EXISTS count는 정확해졌지만, 같은
mailbox에서 여러 메시지를 한 번에 복구하면 중간 count 이벤트가 연속 발행된다.
IMAP EXISTS는 최종 메시지 수만 전달하면 선택된 세션이 정확한 상태로 수렴하므로,
같은 mailbox의 복구 EXISTS 이벤트는 가장 큰 sequence count 하나로 합쳐야 한다.

### 구현 대상

- `internal/mailservice/service.go`
- `internal/mailservice/service_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] UID 기반 EXISTS 이벤트가 mailbox별 가장 큰 sequence count 하나로 합쳐진다.
- [x] 여러 mailbox가 섞인 복구는 mailbox별 EXISTS 이벤트를 유지한다.
- [x] 단건/벌크/스레드 복구 이벤트 테스트가 coalesced EXISTS count를 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-283: IMAP restored EXISTS empty mailbox audit
