# ACTIVE_TASK

## TASK-314: POP3 auth user identity validation consolidation audit

### 배경

POP3 authenticated user ID 검증은 trim, 빈 값 거부, CR/LF 거부를 하나의 계약으로
유지해야 한다. Authenticate 본문에 검증 조건이 직접 흩어지면 이후 POP3 mailbox
생성 경로나 테스트가 일부 조건만 재사용하는 실수가 생길 수 있으므로, 정규화 helper와
직접 테스트로 경계를 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter.go`
- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] authenticated user ID trim/empty/CRLF 검증을 helper로 통합한다.
- [x] helper 단위 테스트로 정상 trim, 빈 값, CR, LF 케이스를 고정한다.
- [x] POP3 Authenticate가 통합 helper를 통해 정규화된 user ID만 사용한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-315: POP3 auth credential validation consolidation audit
