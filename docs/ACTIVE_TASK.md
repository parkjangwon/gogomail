# ACTIVE_TASK

## TASK-456: SMTP submission RSET DSN reset audit

### 배경

SMTP submission `RSET`/`Reset` must clear the active envelope and RFC 3461 DSN options while keeping
the authenticated user available for the next transaction. Explicit reset coverage prevents stale
`RET`, `ENVID`, `NOTIFY`, and `ORCPT` metadata from leaking into later submitted messages.

### 구현 대상

- `internal/smtp/submission_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] DSN envelope/recipient 옵션이 설정된 제출 트랜잭션에서 `Reset` 후 기존 envelope가 사라지는지 검증한다.
- [x] `Reset` 후 같은 인증 세션에서 새 `MAIL`/`RCPT`/`DATA` 제출이 가능한지 검증한다.
- [x] `Reset` 전 `RET`, `ENVID`, `NOTIFY`, `ORCPT` 옵션이 제출 메시지에 누출되지 않는지 검증한다.
- [x] `go test -count=1 ./internal/smtp -run TestSubmissionResetClearsDSNOptions` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-457: SMTP submission MAIL DSN reset audit
