# ACTIVE_TASK

## TASK-389: POP3 DELE empty command preserves pending delete audit

### 배경

POP3 transaction 상태에서 빈 명령 라인은 syntax error만 반환해야 하며 maildrop
상태를 변경하지 않아야 한다. `DELE`로 표시한 pending delete가 parser 오류 처리
때문에 복구되지 않는지 wire-level로 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DELE 1` 이후 빈 명령 라인이 `-ERR syntax error`를 반환하는지 검증한다.
- [x] 빈 명령 이후 `LIST 1`이 계속 `-ERR`를 반환해 pending delete가 유지되는지 검증한다.
- [x] 빈 명령 이후 `STAT`이 삭제 표시된 메시지를 제외하는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-390: POP3 DELE transaction AUTH denial preserves pending delete audit
