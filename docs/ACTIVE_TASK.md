# ACTIVE_TASK

## TASK-354: POP3 AUTH LOGIN password cancellation capability audit

### 배경

POP3 `AUTH LOGIN` password challenge 중 클라이언트가 `*`로 취소해도 서버는
authorization 상태를 유지해야 한다. username 취소 경로와 동일하게 취소 직후 CAPA와
로그인 흐름을 함께 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `AUTH LOGIN` password challenge 취소가 `-ERR authentication cancelled`를 반환하는지 검증한다.
- [x] password 취소 직후 CAPA에 `USER`와 `SASL PLAIN LOGIN`이 남아 authorization 상태를 유지하는지 검증한다.
- [x] 취소 이후 `USER/PASS`와 `STAT`이 성공해 세션이 계속 사용 가능한지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-355: POP3 AUTH PLAIN invalid base64 capability audit
