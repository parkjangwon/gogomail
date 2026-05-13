# ACTIVE_TASK

## TASK-438: IMAP subscription case-insensitive retained-name audit

### 배경

IMAP mailbox names are case-insensitive for common system and retained subscription matching in this
repository. Missing mailbox subscription names are stored with a canonical lower-case key, so a
retained name such as `Retired` must be removable with `retired`. This prevents stale LSUB entries
when clients change mailbox name casing between subscribe and unsubscribe commands.

### 구현 대상

- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] missing mailbox retained subscription을 생성하는 경로를 검증한다.
- [x] retained name과 다른 casing으로 `UnsubscribeIMAPMailbox`가 같은 canonical subscription을 제거하는지 검증한다.
- [x] case-insensitive unsubscribe 후 `ListSubscribedIMAPMailboxes`가 빈 목록을 반환하는지 검증한다.
- [x] `go test -count=1 ./internal/maildb -run TestPostgresIMAPMailboxSubscriptionUnsubscribesRetainedNameCaseInsensitively` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-439: IMAP existing mailbox unsubscribe case-insensitive audit
