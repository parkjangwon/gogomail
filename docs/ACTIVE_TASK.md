# ACTIVE_TASK

## TASK-458: SMTP submission RCPT DSN recipient isolation audit

### 배경

SMTP submission `RCPT TO` with DSN recipient parameters (RFC 3461) must properly validate and isolate per-recipient DSN state.
The `RCPT TO` command accepts optional DSN parameters: `NOTIFY` and `ORCPT` on each recipient.
Coverage must verify that recipient-specific DSN options are correctly isolated and don't leak between RCPT commands
or between transactions.

### 구현 대상

- `internal/smtp/submission_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`

### 완료 조건

- [ ] Multiple `RCPT TO` commands에서 recipient-specific DSN 옵션(NOTIFY, ORCPT)이 정확하게 추적되는지 검증한다.
- [ ] 한 RCPT에 설정된 DSN 옵션이 다음 RCPT에 누출되지 않는지 검증한다.
- [ ] NOTIFY 파라미터 다양성 테스트: NEVER, SUCCESS, FAILURE, DELAY 조합을 검증한다.
- [ ] ORCPT (Original Recipient) 파라미터가 올바르게 인코딩되고 보존되는지 검증한다.
- [ ] 같은 recipient를 여러 번 RCPT할 때 마지막 DSN 옵션이 적용되는지 검증한다.
- [ ] `go test -count=1 ./internal/smtp -run TestSubmissionRcptDSN` 통과.
- [ ] `go test ./...` 통과.
- [ ] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-459: SMTP submission DSN audit completion
