# ACTIVE_TASK

## TASK-447: SMTP submission authenticated hook failure isolation audit

### 배경

SMTP submission emits `StageAuthenticated` hooks during AUTH. If such a hook fails, AUTH must fail
and the session must remain unauthenticated. The previous ordering stored `s.user` before the hook,
so a hook failure could leave a session authenticated even though AUTH returned an error.

### 구현 대상

- `internal/smtp/submission.go`
- `internal/smtp/submission_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] SMTP submission auth hook이 실패하면 session user를 설정하지 않도록 수정한다.
- [x] `AUTH PLAIN`이 hook error를 반환하고 session user가 빈 상태로 남는지 검증한다.
- [x] hook failure 이후 `MAIL FROM`이 `ErrAuthRequired`를 반환하는지 검증한다.
- [x] `go test -count=1 ./internal/smtp -run TestSubmissionAuthHookFailureLeavesSessionUnauthenticated` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-448: SMTP submission auth hook failure metrics audit
