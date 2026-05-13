# ACTIVE_TASK

## TASK-445: SMTP submission must-change-password policy audit

### 배경

SMTP submission authentication receives `MustChangePassword` from the shared gogomail DB-backed
submission authenticator. The session previously accepted that user once credentials were valid,
which allowed password-change-required accounts to submit mail before completing the required reset.
Reject such authentication attempts and keep the SMTP session unauthenticated.

### 구현 대상

- `internal/smtp/submission.go`
- `internal/smtp/submission_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] SMTP submission `AUTH PLAIN`이 `MustChangePassword=true` user를 `ErrAuthFailed`로 거절하도록 수정한다.
- [x] 거절 후 session user가 설정되지 않고 subsequent `MAIL FROM`이 `ErrAuthRequired`를 유지하는지 검증한다.
- [x] auth metric이 rejected로 기록되는지 검증한다.
- [x] `go test -count=1 ./internal/smtp -run TestSubmissionRejectsMustChangePasswordUser` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-446: SMTP submission must-change-password event isolation audit
