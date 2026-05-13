# ACTIVE_TASK

## TASK-430: IMAP bulk move/delete stale-row audit

### 배경

IMAP UID가 할당된 여러 메시지를 일반 maildb `BulkMoveMessages` 또는 `BulkDeleteMessages`로
변경할 때 기존 `imap_message_uid` row가 남으면 old mailbox stale UID 조회/할당 경합이
발생할 수 있다. bulk move/delete 후 모든 대상 메시지의 UID row 제거와 old mailbox 재할당
거절을 Postgres 통합 테스트로 고정한다.

### 구현 대상

- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `BulkMoveMessages` 후 source mailbox의 모든 message-specific `imap_message_uid` row가 제거되는지 검증한다.
- [x] `BulkMoveMessages` 후 old mailbox `EnsureIMAPMessageUID`가 각 메시지에 `ErrIMAPMessageNotActive`를 반환하는지 검증한다.
- [x] moved message들이 destination mailbox에서 fresh UID를 재할당받고 message-specific UID row를 하나씩 유지하는지 검증한다.
- [x] `BulkDeleteMessages` 후 모든 message-specific `imap_message_uid` row가 제거되고 재할당이 거절되는지 검증한다.
- [x] `go test -count=1 ./internal/maildb -run 'TestPostgresBulk(MoveMessagesRemovesOldIMAPUIDRows|DeleteMessagesRemovesIMAPUIDRows)'` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-431: IMAP bulk thread move/delete stale-row audit
