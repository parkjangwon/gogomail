# ACTIVE_TASK

## TASK-434: Mailbox delete deleted-message FK guard audit

### 배경

`DeleteFolder`는 기존에 active 메시지가 없으면 삭제를 시도했지만, soft-deleted messages는
여전히 `messages.folder_id` FK로 folders를 참조한다. 이 상태에서 폴더 삭제를 시도하면
사용자에게 "비어 있지 않음" 대신 DB foreign-key 오류가 노출될 수 있다. 폴더 삭제 전 모든
message status를 기준으로 비어 있는지 판단하고, deleted message가 남은 폴더는 깔끔한
not-empty 오류를 반환하도록 고정한다.

### 구현 대상

- `internal/maildb/messages.go`
- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DeleteFolder`가 active messages뿐 아니라 soft-deleted messages가 남은 폴더도 not-empty로 판단하도록 수정한다.
- [x] deleted message의 IMAP UID row가 제거된 상태에서도 폴더 삭제가 FK 오류를 노출하지 않는지 검증한다.
- [x] `DeleteFolder`가 deleted message 보유 폴더에 깔끔한 `not found or not empty` 오류를 반환하는지 검증한다.
- [x] `go test -count=1 ./internal/maildb -run TestPostgresDeleteFolderRejectsDeletedMessagesWithoutFKError` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-435: IMAP mailbox state cascade cleanup audit
