# ACTIVE_TASK

## TASK-455: SMTP submission Logout domain policy reset audit

### 배경

SMTP submission domain policy caches are keyed by the authenticated user's domain. If a session is
logged out and reauthenticated as a different domain user through the same session object, the old
domain policy and DSN envelope state must not affect the new authenticated identity.

### 구현 대상

- `internal/smtp/submission_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `Logout` 이후 재인증 전에는 `MAIL FROM`이 계속 인증 필요 오류로 거절되는지 검증한다.
- [x] 다른 도메인 사용자로 재인증하면 이전 도메인 정책 캐시가 적용되지 않는지 검증한다.
- [x] `Logout` 전 DSN envelope/recipient 옵션이 재인증 후 제출 메시지에 누출되지 않는지 검증한다.
- [x] `go test -count=1 ./internal/smtp -run TestSubmissionLogoutResetsDomainPolicyForReauth` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-456: SMTP submission RSET DSN reset audit
