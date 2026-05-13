# ACTIVE_TASK

## TASK-439: IMAP existing mailbox unsubscribe case-insensitive audit

### 배경

IMAP mailbox names are case-insensitive for common system mailbox matching. Existing mailbox
subscriptions such as `INBOX` must be removable with a differently cased name such as `inbox`;
otherwise clients that change casing between SUBSCRIBE and UNSUBSCRIBE can leave stale LSUB entries.

### 구현 대상

- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] existing `INBOX` mailbox subscription을 생성하는 경로를 검증한다.
- [x] `inbox` casing으로 `UnsubscribeIMAPMailbox`가 같은 existing mailbox subscription을 제거하는지 검증한다.
- [x] case-insensitive existing mailbox unsubscribe 후 `ListSubscribedIMAPMailboxes`가 빈 목록을 반환하는지 검증한다.
- [x] `go test -count=1 ./internal/maildb -run TestPostgresIMAPMailboxSubscriptionUnsubscribesExistingMailboxCaseInsensitively` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-440: IMAP subscription duplicate casing update audit
