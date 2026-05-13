# ACTIVE_TASK

## TASK-435: IMAP mailbox state cascade cleanup audit

### 배경

IMAP mailbox state는 `folders` row에 cascade FK로 연결되어 있지만, user folder 삭제 후
`imap_mailbox_state` row가 남으면 삭제된 mailbox의 UIDVALIDITY/UIDNEXT 상태가 stale data로
남을 수 있다. 빈 user folder 삭제가 mailbox state를 함께 제거하고 IMAP mailbox 조회가
삭제된 folder를 다시 노출하지 않는지 Postgres 통합 테스트로 고정한다.

### 구현 대상

- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] user folder에 IMAP mailbox state를 만든 뒤 `DeleteFolder`로 folder를 삭제하는 경로를 검증한다.
- [x] folder 삭제 후 `imap_mailbox_state` row가 cascade로 제거되는지 검증한다.
- [x] folder 삭제 후 `GetIMAPMailbox`가 삭제된 mailbox를 다시 노출하지 않는지 검증한다.
- [x] `go test -count=1 ./internal/maildb -run TestPostgresDeleteFolderRemovesIMAPMailboxState` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-436: IMAP mailbox subscription deleted-folder noselect audit
