# ACTIVE_TASK

## TASK-351: POP3 STLS unavailable auth-state session audit

### 배경

TLS 설정이 없는 POP3 서버는 authorization 상태에서도 `STLS`를 거부해야 한다.
이 거부는 TLS 협상을 시작하지 않는 단순 명령 오류이므로, 이후 `USER/PASS` 로그인과
transaction 명령이 계속 정상 동작하는지 wire-level로 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] TLS 설정이 없는 authorization 상태의 `STLS`가 `-ERR`로 거부되는지 검증한다.
- [x] 거부 메시지가 STLS unavailable 사유를 담는지 검증한다.
- [x] 거부 이후 `USER/PASS`와 `STAT`이 성공해 세션이 계속 사용 가능한지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-352: POP3 AUTH PLAIN cancellation session audit
