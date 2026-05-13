# ACTIVE_TASK

## TASK-366: POP3 AUTH PLAIN wrong password capability audit

### 배경

POP3 `AUTH PLAIN` initial response에서 credential 포맷은 정상이지만 인증이 실패하면
서버는 authorization 상태를 유지해야 한다. 실패한 로그인 시도가 capability나 후속
로그인 흐름을 오염시키지 않는지 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `AUTH PLAIN` initial response wrong password가 `-ERR authentication failed`를 반환하는지 검증한다.
- [x] 인증 실패 직후 CAPA에 `USER`와 `SASL PLAIN LOGIN`이 남아 authorization 상태를 유지하는지 검증한다.
- [x] 실패 이후 `USER/PASS`와 `STAT`이 성공해 세션이 계속 사용 가능한지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-367: POP3 AUTH PLAIN challenge wrong password capability audit
