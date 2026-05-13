# ACTIVE_TASK

## TASK-384: POP3 authorization NOOP stability audit

### 배경

POP3 authorization 상태의 `NOOP`은 세션 상태를 변경하지 않아야 한다. 반복 `NOOP`
이후에도 capability와 인증 흐름이 그대로 유지되는지 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] authorization 상태에서 반복 `NOOP`이 `+OK`를 반환하는지 검증한다.
- [x] 반복 `NOOP` 이후 CAPA에 인증 capability가 유지되는지 검증한다.
- [x] 반복 `NOOP` 이후 `USER/PASS`와 `STAT`이 성공해 인증 흐름이 유지되는지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-385: POP3 transaction NOOP stability audit
