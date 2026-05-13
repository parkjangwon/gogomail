# ACTIVE_TASK

## TASK-371: POP3 PASS without USER capability audit

### 배경

POP3 authorization 상태에서 `USER` 없이 `PASS`가 들어오면 인증 실패로 처리되어야
하며 서버는 authorization 상태를 유지해야 한다. 순서 오류가 capability나 후속 로그인
흐름을 오염시키지 않는지 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `USER` 없이 `PASS`가 `-ERR authentication failed`를 반환하는지 검증한다.
- [x] 순서 오류 직후 CAPA에 `USER`와 `SASL PLAIN LOGIN`이 남아 authorization 상태를 유지하는지 검증한다.
- [x] 오류 이후 `USER/PASS`와 `STAT`이 성공해 세션이 계속 사용 가능한지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-372: POP3 USER replacement before PASS audit
