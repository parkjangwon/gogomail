# ACTIVE_TASK

## TASK-450: SMTP submission malformed auth payload isolation audit

### 배경

Malformed SMTP submission AUTH PLAIN payloads fail inside the SASL parser before the authenticator
callback runs. Such malformed payloads must not authenticate the session and must not emit
authenticated hook events, so downstream audit/logging extensions never observe a principal for a
parse-failed AUTH attempt.

### 구현 대상

- `internal/smtp/submission_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] malformed SMTP submission AUTH PLAIN payload가 오류를 반환하는지 검증한다.
- [x] malformed payload 이후 session user가 빈 상태로 남는지 검증한다.
- [x] malformed payload 거절 경로에서 auth hook event가 하나도 방출되지 않는지 검증한다.
- [x] malformed payload 이후 `MAIL FROM`이 `ErrAuthRequired`를 반환하는지 검증한다.
- [x] `go test -count=1 ./internal/smtp -run TestSubmissionMalformedAuthPayloadLeavesSessionUnauthenticated` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-451: SMTP submission unsupported auth mechanism audit
