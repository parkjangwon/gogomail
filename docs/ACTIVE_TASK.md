# ACTIVE_TASK

## TASK-317: POP3 password passthrough preservation audit

### 배경

POP3 password는 CR/LF만 금지하고, 그 외 문자는 authenticator에 원문 그대로 전달해야
한다. 비밀번호 앞뒤 공백은 실제 비밀번호의 일부일 수 있으므로 adapter가 trim하거나
변형하면 사용자 인증 데이터와 서버 동작이 틀어진다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 공백 포함 password로 Authenticate를 호출해 성공 경로를 검증한다.
- [x] authenticator가 공백 포함 password 원문을 그대로 받는지 검증한다.
- [x] 기존 username normalization 기록 필드와 충돌하지 않는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-318: POP3 invalid credential short-circuit audit
