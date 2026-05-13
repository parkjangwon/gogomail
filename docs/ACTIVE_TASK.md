# ACTIVE_TASK

## TASK-391: POP3 DELE transaction USER PASS denial preserves pending delete audit

### 배경

POP3 transaction 상태에서 `USER`/`PASS` 재인증 시도는 거부되어야 하며 maildrop
상태를 변경하지 않아야 한다. `DELE`로 표시한 pending delete가 USER/PASS 거부 처리
때문에 복구되지 않는지 wire-level로 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DELE 1` 이후 transaction `USER`가 `-ERR unknown command`를 반환하는지 검증한다.
- [x] `DELE 1` 이후 transaction `PASS`가 `-ERR unknown command`를 반환하는지 검증한다.
- [x] USER/PASS 거부 이후 `LIST 1`과 `STAT`으로 pending delete가 유지되는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-392: POP3 DELE STLS denial preserves pending delete audit
