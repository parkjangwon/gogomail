# ACTIVE_TASK

## TASK-350: POP3 STLS transaction-state denial audit

### 배경

POP3 `STLS`는 authorization 상태에서만 허용되어야 한다. 인증 후 transaction
상태에서 `STLS`가 들어오면 TLS 협상을 시작하지 않고 명확히 거부하되 기존 POP3
세션은 계속 사용할 수 있어야 하므로 wire-level 회귀로 분리해 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 인증 후 transaction 상태의 `STLS`가 `-ERR`로 거부되는지 검증한다.
- [x] 거부 메시지가 transaction-state STLS 불가 사유를 담는지 검증한다.
- [x] 거부 이후 `NOOP`과 `STAT`이 성공해 세션이 계속 사용 가능한지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-351: POP3 STLS unavailable auth-state session audit
