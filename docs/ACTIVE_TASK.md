# ACTIVE_TASK

## TASK-446: SMTP submission must-change-password event isolation audit

### 배경

After SMTP submission rejects a `MustChangePassword` user, downstream hooks must not receive a
`StageAuthenticated` success event. Otherwise audit, logging, or policy extensions could observe a
false successful authentication even though the SMTP session remains unauthenticated.

### 구현 대상

- `internal/smtp/submission_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `MustChangePassword=true` user의 SMTP submission `AUTH PLAIN` 거절 경로를 hook-enabled receiver에서 검증한다.
- [x] 거절 후 hook stages에 `StageAuthenticated` 또는 다른 성공 이벤트가 기록되지 않는지 검증한다.
- [x] `go test -count=1 ./internal/smtp -run TestSubmissionDoesNotEmitAuthHookForMustChangePasswordUser` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-447: SMTP submission authenticated hook failure isolation audit
