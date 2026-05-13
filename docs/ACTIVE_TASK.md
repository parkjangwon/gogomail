# ACTIVE_TASK

## TASK-313: POP3 auth control-character identity rejection audit

### 배경

POP3 gateway는 클라이언트 username/password에서 CR/LF를 거부하지만, authenticator가
반환한 user ID도 mail service 조회와 maildrop lock key의 신뢰 경계에 들어간다.
인증된 identity 안의 CR/LF는 로그/프로토콜/조회 경계에서 혼선을 만들 수 있으므로,
서비스 조회 전에 POP3 adapter가 명확히 거부해야 한다.

### 구현 대상

- `internal/mailservice/pop3_adapter.go`
- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] CR/LF가 포함된 authenticated user ID를 POP3 adapter가 거부한다.
- [x] CR/LF가 포함된 authenticated user ID가 folder 조회로 전달되지 않는지 검증한다.
- [x] CR/LF가 포함된 authenticated user ID가 inbox page 조회로 전달되지 않는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-314: POP3 auth user identity validation consolidation audit
