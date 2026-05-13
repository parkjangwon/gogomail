# ACTIVE_TASK

## TASK-284: Mail stored IMAP EXISTS message count audit

### 배경

수신/저장된 메시지는 `mail.stored` 이벤트 처리에서 `EnsureIMAPMessageUID`로
UID/sequence를 보장한 뒤 IMAP EXISTS 이벤트를 발행한다. 복구 경로는
`Messages=SequenceNumber`를 포함하도록 보강됐지만, 수신 알림 경로는
`Messages=0`인 이벤트를 발행해 선택된 세션이 단순 증가에 의존했다. 수신
알림도 정확한 mailbox count를 전달해야 APPEND/COPY/RESTORE와 같은 EXISTS
수렴 규칙을 공유한다.

### 구현 대상

- `internal/imapnotify/handler.go`
- `internal/imapnotify/handler_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `mail.stored` IMAP EXISTS 이벤트가 `Messages=SequenceNumber`를 포함한다.
- [x] 수신 알림 publish 테스트가 EXISTS count를 검증한다.
- [x] `go test ./internal/imapnotify` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-285: Mail stored IMAP empty mailbox event audit
