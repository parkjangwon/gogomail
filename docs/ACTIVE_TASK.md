# ACTIVE_TASK

## TASK-356: POP3 AUTH PLAIN invalid format capability audit

### 배경

POP3 `AUTH PLAIN`에서 base64 디코딩은 성공했지만 credential 필드 포맷이 틀려도
서버는 authorization 상태를 유지해야 한다. 포맷 오류가 세션 상태나 capability를
오염시키지 않는지 CAPA와 일반 로그인 흐름으로 고정한다.

### 구현 대상

- `internal/pop3d/pop3d_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] invalid format `AUTH PLAIN` 초기 응답이 `-ERR invalid credentials format`을 반환하는지 검증한다.
- [x] 포맷 오류 직후 CAPA에 `USER`와 `SASL PLAIN LOGIN`이 남아 authorization 상태를 유지하는지 검증한다.
- [x] 오류 이후 `USER/PASS`와 `STAT`이 성공해 세션이 계속 사용 가능한지 검증한다.
- [x] `go test ./internal/pop3d` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-357: POP3 AUTH LOGIN invalid username base64 capability audit
