# ACTIVE_TASK

## TASK-312: POP3 auth empty user identity rejection audit

### 배경

POP3 gateway는 authenticator가 반환한 user ID를 mail service 조회와 maildrop lock
key의 신뢰 경계로 사용한다. 인증은 성공했지만 identity adapter가 빈 user ID를
반환하면 POP3 세션은 빈 ID로 folder/page 조회를 시작하면 안 되며, 인증 경계에서
명확히 실패해야 한다.

### 구현 대상

- `internal/mailservice/pop3_adapter.go`
- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] trim 후 빈 authenticated user ID를 POP3 adapter가 거부한다.
- [x] 빈 authenticated user ID가 folder 조회로 전달되지 않는지 검증한다.
- [x] 빈 authenticated user ID가 inbox page 조회로 전달되지 않는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-313: POP3 auth control-character identity rejection audit
