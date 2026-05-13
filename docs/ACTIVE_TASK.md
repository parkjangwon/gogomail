# ACTIVE_TASK

## TASK-379: POP3 transaction USER PASS denial audit

### 배경

POP3 transaction 상태에서는 재인증용 `USER`/`PASS` 명령을 허용하지 않아야 한다.
잘못된 재인증 시도가 기존 maildrop 세션을 오염시키지 않고 이후 명령을 계속 처리하는지
고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] transaction 상태의 `USER`가 `-ERR unknown command`로 거부되는지 검증한다.
- [x] transaction 상태의 `PASS`가 `-ERR unknown command`로 거부되는지 검증한다.
- [x] 거부 이후 `NOOP`과 `STAT`이 성공해 기존 transaction 세션이 유지되는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-380: POP3 transaction AUTH denial audit
