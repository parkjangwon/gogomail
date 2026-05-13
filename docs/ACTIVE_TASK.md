# ACTIVE_TASK

## TASK-448: SMTP submission auth hook failure metrics audit

### 배경

When SMTP submission AUTH fails because a `StageAuthenticated` hook rejects the event, metrics must
record that auth attempt as rejected and must not produce an accepted auth metric. This keeps
operational dashboards and security monitoring aligned with the actual AUTH result.

### 구현 대상

- `internal/smtp/submission_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] SMTP submission auth hook failure가 `StageAuthenticated` rejected metric을 기록하는지 검증한다.
- [x] 같은 auth hook failure가 accepted auth metric을 기록하지 않는지 검증한다.
- [x] `go test -count=1 ./internal/smtp -run TestSubmissionAuthHookFailureRecordsRejectedMetric` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-449: SMTP submission invalid credentials event isolation audit
