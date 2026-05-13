# ACTIVE_TASK

## TASK-323: POP3 missing inbox service short-circuit audit

### 배경

POP3는 INBOX가 있어야 mailbox를 열 수 있다. folder 목록 조회는 필요하지만 INBOX가
없으면 message page 조회를 시도하면 안 된다. 잘못된 folder ID나 빈 ID로 message
listing이 진행되지 않도록 missing inbox 경로의 service short-circuit을 테스트로
고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] INBOX가 없는 folder 목록에서는 인증 후 missing inbox 오류가 발생하는지 검증한다.
- [x] INBOX가 없는 경우 folder 조회는 정규화된 user ID로 한 번 수행되는지 검증한다.
- [x] INBOX가 없는 경우 message page 조회가 수행되지 않는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-324: POP3 folder listing error short-circuit audit
