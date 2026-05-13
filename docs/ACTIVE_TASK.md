# ACTIVE_TASK

## TASK-329: POP3 empty inbox pagination audit

### 배경

POP3 Authenticate는 빈 INBOX도 정상 mailbox로 열어야 한다. 메시지가 없을 때 page
조회가 반복되거나 missing inbox와 혼동되면 빈 메일함 사용자가 로그인에 실패할 수
있으므로, empty inbox pagination 경로를 테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 빈 INBOX로 POP3 Authenticate가 성공하는지 검증한다.
- [x] 빈 INBOX mailbox의 message count가 0인지 검증한다.
- [x] 빈 INBOX page 조회가 zero cursor로 한 번만 수행되는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-330: POP3 message size conversion audit
