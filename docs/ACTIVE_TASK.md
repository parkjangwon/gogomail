# ACTIVE_TASK

## TASK-332: POP3 invalid message index size audit

### 배경

POP3 server는 command sequence number를 mailbox index로 변환해 `MessageSize`를 호출한다.
잘못된 index가 전달되더라도 mailbox는 panic하거나 음수 값을 반환하면 안 되고, 기존
계약대로 0을 반환해야 한다. 이 경계 동작을 명시 테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `MessageSize(-1)`이 0을 반환하는지 검증한다.
- [x] `MessageSize(MessageCount())`가 0을 반환하는지 검증한다.
- [x] invalid size 조회가 기존 mailbox 상태를 요구하지 않고 안전하게 완료되는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-333: POP3 invalid UIDL index audit
