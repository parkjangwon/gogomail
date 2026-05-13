# ACTIVE_TASK

## TASK-437: IMAP subscription name normalization audit

### 배경

IMAP subscription API가 mailbox name/id 입력의 앞뒤 공백을 그대로 canonical key와 retained
name에 반영하면 `" INBOX "` 같은 입력이 실제 `INBOX`가 아니라 missing mailbox 구독으로
저장될 수 있다. repository 경계에서 mailbox name/id를 trim하고, canonical subscription name도
trim 기준으로 계산해 기존 mailbox 조회와 unsubscribe가 같은 이름으로 동작하도록 고정한다.

### 구현 대상

- `internal/maildb/imap_subscriptions.go`
- `internal/maildb/imap_subscriptions_test.go`
- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `SubscribeIMAPMailbox`가 mailbox name/id 입력의 앞뒤 공백을 제거한 뒤 기존 mailbox를 조회하도록 수정한다.
- [x] `UnsubscribeIMAPMailbox`도 같은 trim 규칙으로 retained/existing subscription을 제거하도록 수정한다.
- [x] canonical subscription name 계산이 trim된 이름 기준으로 동작하도록 수정한다.
- [x] canonical subscription unit test를 trim 기반 정규화 계약으로 갱신한다.
- [x] `" INBOX "` 구독이 missing mailbox가 아니라 기존 Inbox 구독으로 저장되고 같은 입력으로 unsubscribe되는지 검증한다.
- [x] `go test -count=1 ./internal/maildb -run TestPostgresIMAPMailboxSubscriptionTrimsNames` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-438: IMAP subscription case-insensitive retained-name audit
