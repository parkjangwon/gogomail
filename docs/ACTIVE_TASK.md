# ACTIVE_TASK

## TASK-428: IMAP message UID row-lock audit

### 배경

IMAP 단일 메시지 UID 할당은 mailbox state, folder, target message row 순서로 lock을 잡아야
한다. target message row lock이 빠지면 concurrent move/delete와 stale UID insert가 경합할 수
있으므로, 실제 SQL과 Postgres 통합 테스트가 이 보장을 고정하는지 감사한다.

### 구현 대상

- `internal/maildb/imap_uid.go`
- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `EnsureIMAPMessageUID` SQL이 target `messages` row를 `FOR UPDATE OF m`으로 lock하는지 확인한다.
- [x] Postgres 통합 테스트가 locked message row에서 `EnsureIMAPMessageUID` 대기를 검증하는지 확인한다.
- [x] lock 대기 중 stale UID row가 생성되지 않는지 확인한다.
- [x] lock 해제 후 UID/MODSEQ 1/1 할당이 성공하는지 확인한다.
- [x] `go test -count=1 ./internal/maildb -run TestPostgresEnsureIMAPMessageUIDWaitsForMessageRowLock` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-429: IMAP message UID move/delete stale-row audit
