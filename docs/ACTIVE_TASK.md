# ACTIVE_TASK

## TASK-316: POP3 username normalization passthrough audit

### 배경

POP3 adapter는 username을 trim한 뒤 authenticator에 전달해야 한다. helper 단위
테스트만으로는 실제 Authenticate 경로가 정규화된 username을 사용하는지 놓칠 수
있으므로, 인증 mock이 받은 username을 기록해 adapter 경계의 passthrough 동작을
고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 test authenticator가 전달받은 username을 기록한다.
- [x] 공백 포함 username으로 Authenticate를 호출해 authenticator가 trim된 username만 받는지 검증한다.
- [x] 기존 POP3 adapter 테스트가 기록 필드 추가 후에도 통과한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-317: POP3 password passthrough preservation audit
