# ACTIVE_TASK

## TASK-318: POP3 invalid credential short-circuit audit

### 배경

POP3 adapter에서 username/password 형식 검증이 실패한 요청은 authenticator나
mail service까지 내려가면 안 된다. invalid credential을 backend로 전달하면 불필요한
감사/로그/부하가 생기고, backend마다 다르게 해석될 수 있으므로 adapter에서 즉시
차단되는지 테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 빈 username이 authenticator 호출 없이 거부되는지 검증한다.
- [x] CR/LF 포함 username이 authenticator 호출 없이 거부되는지 검증한다.
- [x] CR/LF 포함 password가 authenticator 호출 없이 거부되는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-319: POP3 must-change-password service short-circuit audit
