# ACTIVE_TASK

## TASK-322: POP3 inbox folder first-match audit

### 배경

POP3 adapter는 사용자의 folder 목록에서 첫 번째 `inbox` system folder를 mailbox로
선택한다. 데이터 복구나 중복 설정 상황에서도 선택 규칙이 흔들리면 POP3가 다른 folder를
노출할 수 있으므로, page 조회에 전달된 folder ID를 기록해 first-match 동작을 테스트로
고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 test repository가 inbox page 조회의 folder ID를 기록한다.
- [x] non-inbox 및 중복 inbox folder 목록에서 첫 번째 inbox folder를 선택하는지 검증한다.
- [x] 선택된 첫 번째 inbox folder로 message count가 로드되는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-323: POP3 missing inbox service short-circuit audit
