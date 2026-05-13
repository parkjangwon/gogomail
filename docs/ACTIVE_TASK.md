# ACTIVE_TASK

## TASK-333: POP3 invalid UIDL index audit

### 배경

POP3 UIDL 조회도 command sequence number에서 mailbox index로 변환된 값을 사용한다.
잘못된 index가 전달되더라도 UIDL은 panic하거나 임의 메시지 ID를 반환하면 안 되고,
기존 계약대로 빈 문자열을 반환해야 한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `MessageUIDL(-1)`이 빈 문자열을 반환하는지 검증한다.
- [x] `MessageUIDL(MessageCount())`가 빈 문자열을 반환하는지 검증한다.
- [x] invalid UIDL 조회가 기존 mailbox 상태를 요구하지 않고 안전하게 완료되는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-334: POP3 invalid content index audit
