# ACTIVE_TASK

## TASK-385: POP3 transaction NOOP stability audit

### 배경

POP3 transaction 상태의 `NOOP`은 maildrop 상태를 변경하지 않아야 한다. 반복 `NOOP`
이후에도 메시지 조회와 상태 조회가 그대로 유지되는지 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] transaction 상태에서 반복 `NOOP`이 `+OK`를 반환하는지 검증한다.
- [x] 반복 `NOOP` 이후 `LIST 1`이 기존 메시지 크기를 반환하는지 검증한다.
- [x] 반복 `NOOP` 이후 `STAT`이 기존 maildrop 상태를 반환하는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-386: POP3 DELE NOOP preserves pending delete audit
