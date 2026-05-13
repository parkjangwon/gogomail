# ACTIVE_TASK

## TASK-453: SMTP submission repeated auth transaction state audit

### 배경

SMTP submission must reject repeated AUTH after a session is already authenticated, including when a
mail transaction is already in progress. That rejection must not clear the active envelope sender or
otherwise abort a valid transaction that can continue with `RCPT` and `DATA`.

### 구현 대상

- `internal/smtp/submission_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `MAIL FROM` 이후 repeated AUTH가 거절되는지 검증한다.
- [x] repeated AUTH 거절 후 active envelope sender가 보존되는지 검증한다.
- [x] 거절 후 `RCPT`와 `DATA`가 이어져 submitted message가 정상 기록되는지 검증한다.
- [x] `go test -count=1 ./internal/smtp -run TestSubmissionRepeatedAuthPreservesEnvelopeTransaction` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-454: SMTP submission unsupported auth transaction state audit
