# ACTIVE_TASK

## TASK-449: SMTP submission invalid credentials event isolation audit

### 배경

Invalid SMTP submission credentials must fail before any authenticated hook event is emitted. This
keeps audit/logging extensions from observing unauthenticated identities while still recording the
rejected auth metric for operational visibility.

### 구현 대상

- `internal/smtp/submission_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] invalid SMTP submission credentials가 `ErrAuthFailed`로 거절되는지 검증한다.
- [x] invalid credentials 거절 경로에서 auth hook event가 하나도 방출되지 않는지 검증한다.
- [x] invalid credentials 거절 경로에서 rejected auth metric은 기록되는지 검증한다.
- [x] `go test -count=1 ./internal/smtp -run TestSubmissionInvalidCredentialsDoNotEmitAuthHook` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-450: SMTP submission malformed auth payload isolation audit
