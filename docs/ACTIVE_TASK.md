# ACTIVE_TASK

## TASK-392: POP3 DELE STLS denial preserves pending delete audit

### 배경

POP3 transaction 상태에서 `STLS`는 거부되어야 하며 maildrop 상태를 변경하지 않아야
한다. `DELE`로 표시한 pending delete가 STLS 거부 처리 때문에 복구되지 않는지
wire-level로 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DELE 1` 이후 transaction `STLS`가 명확한 `-ERR` 거부를 반환하는지 검증한다.
- [x] STLS 거부 이후 `LIST 1`이 계속 `-ERR`를 반환해 pending delete가 유지되는지 검증한다.
- [x] STLS 거부 이후 `STAT`이 삭제 표시된 메시지를 제외하는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-393: POP3 DELE invalid command sequence preserves pending delete docs audit
