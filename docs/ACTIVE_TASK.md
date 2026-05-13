# ACTIVE_TASK

## TASK-320: POP3 auth failure service short-circuit audit

### 배경

POP3 authenticator가 credential 실패를 반환한 경우 adapter는 generic authentication
failure로 종료하고 mailbox 조회를 시작하면 안 된다. 인증 실패 계정에 대해 folder/page
조회가 발생하면 불필요한 데이터 접근과 계정 존재성 단서가 생길 수 있으므로 service
short-circuit을 테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 인증 실패 시 authenticator가 한 번 호출되는지 검증한다.
- [x] 인증 실패가 folder 조회로 내려가지 않는지 검증한다.
- [x] 인증 실패가 inbox page 조회로 내려가지 않는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-321: POP3 inbox folder casing audit
