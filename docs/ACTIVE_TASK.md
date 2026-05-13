# ACTIVE_TASK

## TASK-451: SMTP submission unsupported auth mechanism audit

### 배경

SMTP submission advertises and supports `AUTH PLAIN` only. Unsupported mechanisms such as `LOGIN`
must return `ErrAuthUnsupported` without creating a SASL server, authenticating the session,
emitting hooks, or recording misleading auth metrics.

### 구현 대상

- `internal/smtp/submission_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] unsupported SMTP submission auth mechanism이 `ErrAuthUnsupported`와 nil SASL server를 반환하는지 검증한다.
- [x] unsupported mechanism 이후 session user가 빈 상태로 남고 `MAIL FROM`이 `ErrAuthRequired`를 반환하는지 검증한다.
- [x] unsupported mechanism 거절 경로에서 hook event와 metric이 기록되지 않는지 검증한다.
- [x] `go test -count=1 ./internal/smtp -run TestSubmissionRejectsUnsupportedAuthMechanismWithoutSideEffects` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-452: SMTP submission repeated auth side-effect isolation audit
