# ACTIVE_TASK

## TASK-373: POP3 USER syntax preserves auth capability audit

### 배경

POP3 authorization 상태에서 `USER` 인자 개수가 틀리면 syntax error를 반환해야 한다.
문법 오류가 pending user나 capability 상태를 오염시키지 않고 정상 로그인 재시도를
허용하는지 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 인자 2개 이상의 `USER`가 `-ERR syntax error`를 반환하는지 검증한다.
- [x] 문법 오류 직후 CAPA에 `USER`와 `SASL PLAIN LOGIN`이 남아 authorization 상태를 유지하는지 검증한다.
- [x] 오류 이후 `USER/PASS`와 `STAT`이 성공해 세션이 계속 사용 가능한지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-374: POP3 PASS syntax preserves auth capability audit
