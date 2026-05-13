# ACTIVE_TASK

## TASK-283: IMAP restored EXISTS empty mailbox audit

### 배경

복구된 메시지는 `EnsureIMAPMessageUIDsForMessages`로 UID/sequence를 보장한 뒤
IMAP EXISTS 이벤트를 발행한다. summary 기반 이벤트 발행은 mailbox ID가 비어
있으면 이벤트를 건너뛰지만, UID 기반 이벤트 발행은 repository 결과에 빈
mailbox ID가 섞여도 그대로 publish할 수 있었다. 잘못된 UID 결과가 브로커에
전파되지 않도록 UID 기반 이벤트도 빈 mailbox ID를 무시해야 한다.

### 구현 대상

- `internal/mailservice/service.go`
- `internal/mailservice/service_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] UID 기반 IMAP 이벤트 발행이 빈 mailbox ID를 publish하지 않는다.
- [x] 복구 EXISTS 이벤트가 빈 mailbox 결과를 건너뛰고 유효한 mailbox 이벤트만 발행한다.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-284: Mail stored IMAP EXISTS message count audit
