# ACTIVE_TASK

## TASK-319: POP3 must-change-password service short-circuit audit

### 배경

POP3 인증은 성공했더라도 `must_change_password` 상태인 계정은 mailbox를 열면 안
된다. 이 정책은 credential 확인 직후 적용되어야 하며, folder/page 조회가 수행되면
정책 적용 전에 계정 데이터 접근이 발생할 수 있으므로 service short-circuit을 테스트로
고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] must-change-password 계정 인증 시 authenticator는 한 번 호출되는지 검증한다.
- [x] must-change-password 계정이 folder 조회로 내려가지 않는지 검증한다.
- [x] must-change-password 계정이 inbox page 조회로 내려가지 않는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-320: POP3 auth failure service short-circuit audit
