# ACTIVE_TASK

## TASK-429: IMAP message UID move/delete stale-row audit

### 배경

IMAP 단일 메시지 UID가 할당된 메시지를 일반 maildb `MoveMessage` 또는 `DeleteMessage`로
변경할 때 기존 `imap_message_uid` row가 남으면 old mailbox stale UID 조회/할당 경합이
발생할 수 있다. move/delete 후 UID row 제거와 old mailbox 재할당 거절을 Postgres 통합
테스트로 고정한다.

### 구현 대상

- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `MoveMessage` 후 source mailbox의 message-specific `imap_message_uid` row가 제거되는지 검증한다.
- [x] `MoveMessage` 후 old mailbox `EnsureIMAPMessageUID`가 `ErrIMAPMessageNotActive`를 반환하는지 검증한다.
- [x] moved message가 destination mailbox에서 fresh UID 1로 재할당되는지 검증한다.
- [x] `DeleteMessage` 후 message-specific `imap_message_uid` row가 제거되고 재할당이 거절되는지 검증한다.
- [x] `go test -count=1 ./internal/maildb -run 'TestPostgres(MoveMessageRemovesOldIMAPUIDRows|DeleteMessageRemovesIMAPUIDRows)'` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-430: IMAP bulk move/delete stale-row audit
