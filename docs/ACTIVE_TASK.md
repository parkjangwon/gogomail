# ACTIVE_TASK

## TASK-325: POP3 message page error propagation audit

### 배경

POP3 Authenticate는 INBOX message page 조회가 실패하면 mailbox를 만들면 안 되고
`list inbox messages` 오류로 실패해야 한다. page 오류를 빈 mailbox처럼 취급하면
사용자는 실제 메일함 장애를 정상적인 빈 INBOX로 오해할 수 있으므로 오류 전파 경로를
테스트로 고정한다.

### 구현 대상

- `internal/mailservice/pop3_adapter_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] POP3 test repository에 message page 오류 주입을 추가한다.
- [x] message page 오류가 `list inbox messages` 오류로 전파되는지 검증한다.
- [x] message page 오류가 선택된 INBOX folder ID로 한 번 발생하는지 검증한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-326: POP3 message page cursor error audit
