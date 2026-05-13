# ACTIVE_TASK

## TASK-358: POP3 AUTH LOGIN invalid password base64 capability audit

### 배경

POP3 `AUTH LOGIN` password challenge에서 invalid base64가 들어와도 서버는
authorization 상태를 유지해야 한다. password 파싱 오류가 세션 상태나 capability를
오염시키지 않는지 CAPA와 일반 로그인 흐름으로 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `AUTH LOGIN` password challenge의 invalid base64가 `-ERR invalid base64`를 반환하는지 검증한다.
- [x] password 오류 직후 CAPA에 `USER`와 `SASL PLAIN LOGIN`이 남아 authorization 상태를 유지하는지 검증한다.
- [x] 오류 이후 `USER/PASS`와 `STAT`이 성공해 세션이 계속 사용 가능한지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-359: POP3 auth capability assertion helper cleanup
