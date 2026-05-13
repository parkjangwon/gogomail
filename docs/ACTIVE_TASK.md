# ACTIVE_TASK

## TASK-452: SMTP submission repeated auth side-effect isolation audit

### 배경

SMTP submission must reject repeated AUTH after a session is already authenticated. That rejection
must not replace or clear the existing authenticated user, emit extra hook events, or record
misleading duplicate auth metrics.

### 구현 대상

- `internal/smtp/submission_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] authenticated SMTP submission session에서 repeated AUTH가 거절되는지 검증한다.
- [x] repeated AUTH 거절 후 기존 authenticated user가 유지되고 `MAIL FROM`이 계속 가능한지 검증한다.
- [x] repeated AUTH 거절 경로에서 hook event와 metric이 추가로 기록되지 않는지 검증한다.
- [x] `go test -count=1 ./internal/smtp -run TestSubmissionRepeatedAuthHasNoSideEffects` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-453: SMTP submission repeated auth transaction state audit
