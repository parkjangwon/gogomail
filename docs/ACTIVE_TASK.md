# ACTIVE_TASK

## TASK-433: IMAP single restore UID reactivation audit

### 배경

IMAP UID가 할당된 메시지를 단일 `DeleteMessage` 후 `RestoreMessage`로 복구할 때 삭제로
제거된 `imap_message_uid` row가 되살아나거나 이전 UID가 재사용되면 IMAP 클라이언트의
expunge/UID 불변성 가정이 깨질 수 있다. single restore 후 복구된 메시지가 기존 UID를
재사용하지 않고 mailbox state 기준 fresh UID를 받는지 Postgres 통합 테스트로 고정한다.

### 구현 대상

- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `DeleteMessage` 후 UID row가 제거된 메시지를 `RestoreMessage`로 복구하면 fresh UID를 새로 할당받는지 검증한다.
- [x] restored message가 delete 전 UID를 재사용하지 않고 기존 mailbox UID보다 큰 UID를 받는지 검증한다.
- [x] restored message가 fresh UID 할당 후 message-specific UID row를 하나 유지하는지 검증한다.
- [x] `go test -count=1 ./internal/maildb -run TestPostgresRestoreMessageAssignsFreshIMAPUID` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-434: IMAP mailbox delete stale UID cleanup audit
