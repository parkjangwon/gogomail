# ACTIVE_TASK

## TASK-372: POP3 USER replacement before PASS audit

### 배경

POP3 authorization 상태에서는 `PASS` 전에 `USER`를 다시 보낼 수 있다. 이때 서버가
이전 사용자 값을 고정하지 않고 마지막 `USER` 값을 기준으로 인증하는지 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `USER bob` 이후 `USER alice`가 `+OK`로 사용자 선택을 갱신하는지 검증한다.
- [x] 이어지는 `PASS secret`이 마지막 `USER alice` 기준으로 인증 성공하는지 검증한다.
- [x] 인증 성공 후 CAPA와 `STAT`으로 transaction 상태 전환을 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-373: POP3 USER syntax preserves auth capability audit
