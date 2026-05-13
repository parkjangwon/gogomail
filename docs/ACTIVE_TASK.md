# ACTIVE_TASK

## TASK-377: POP3 transaction empty command recovery audit

### 배경

POP3 transaction 상태에서 빈 명령 라인은 `-ERR syntax error`로 처리되어야 하며
기존 maildrop 세션은 계속 사용할 수 있어야 한다. 파서 오류가 transaction 상태를
오염시키지 않는지 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] transaction 상태의 빈 명령 라인이 `-ERR syntax error`를 반환하는지 검증한다.
- [x] syntax error 이후 `NOOP`이 성공해 transaction 세션이 계속 사용 가능한지 검증한다.
- [x] syntax error 이후 `STAT`이 성공해 maildrop 상태가 유지되는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-378: POP3 authorization empty command recovery audit
