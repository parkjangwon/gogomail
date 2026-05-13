# ACTIVE_TASK

## TASK-285: Mail stored IMAP empty mailbox event audit

### 배경

수신/저장된 메시지는 `mail.stored` 이벤트 처리에서 `EnsureIMAPMessageUID`로
UID/sequence를 보장한 뒤 IMAP EXISTS 이벤트와 delta sync mailbox 변경 알림을
발행한다. 입력 payload의 folder ID는 검증하지만, UID 보장 결과의 mailbox ID가
비어 있으면 mailbox 없는 IMAP 이벤트나 delta sync 알림이 전파될 수 있다.
복구 UID 이벤트와 동일하게 mail.stored 알림도 빈 mailbox 결과를 조용히
건너뛰어야 한다.

### 구현 대상

- `internal/imapnotify/handler.go`
- `internal/imapnotify/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `mail.stored` UID 결과의 mailbox ID가 비어 있으면 IMAP EXISTS 이벤트를 발행하지 않는다.
- [x] 같은 조건에서 delta sync mailbox 변경 알림도 발행하지 않는다.
- [x] 수신 알림 handler 테스트가 빈 mailbox UID 결과를 검증한다.
- [x] `go test ./internal/imapnotify` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-286: IMAP event broker nil event audit
