# ACTIVE_TASK

## TASK-459: SMTP submission DSN audit completion

### 배경

SMTP submission DSN support has accumulated coverage for `MAIL FROM` DSN parameters, `RCPT TO`
recipient DSN parameters, session resets, logout, queue propagation, and TCP wire-level behavior.
Before moving to the next SMTP hardening item, run a completion audit that verifies the submission DSN
surface is covered end-to-end and document any remaining gap as either fixed coverage or an explicit
follow-up task.

### 구현 대상

- `internal/smtp/submission_test.go`
- `internal/smtp/protocol_integration_test.go`
- `internal/mailflow/*_test.go`
- `internal/delivery/*_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`

### 완료 조건

- [x] Submission DSN audit checklist를 작성하고 `MAIL FROM` `RET`/`ENVID`, `RCPT TO` `NOTIFY`/`ORCPT`, reset/logout/new transaction isolation, and wire-level DSN propagation coverage를 실제 테스트와 매핑한다.
- [x] 누락된 submission DSN edge case가 있으면 실패 테스트를 먼저 추가하고 구현/수정한다.
- [x] `go test -count=1 ./internal/smtp -run 'TestSubmission.*DSN|TestSMTPProtocol.*DSN'` 통과.
- [x] `go test -count=1 ./internal/mailflow ./internal/delivery -run DSN` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TBD after audit.
