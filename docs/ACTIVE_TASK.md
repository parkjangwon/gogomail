# ACTIVE_TASK

## TASK-440: IMAP subscription duplicate casing update audit

### 배경

IMAP subscription canonical keys are lower-case, so repeated SUBSCRIBE commands for a retained
missing mailbox with different casing must update the same row rather than creating duplicates.
This prevents duplicate LSUB output and confirms the stored display name follows the latest
subscription command for retained names.

### 구현 대상

- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] missing mailbox retained subscription을 한 casing으로 생성한 뒤 다른 casing으로 다시 subscribe하는 경로를 검증한다.
- [x] duplicate casing subscribe가 subscription row를 늘리지 않고 canonical key 하나만 유지하는지 검증한다.
- [x] retained display name이 최신 subscribe 명령의 name으로 갱신되는지 검증한다.
- [x] `go test -count=1 ./internal/maildb -run TestPostgresIMAPMailboxSubscriptionDuplicateCasingUpdatesRetainedName` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-441: IMAP mailbox list subscription ordering audit
