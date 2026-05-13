# ACTIVE_TASK

## TASK-277: IMAP lazy UID capacity race audit

### 배경

IMAP UID capacity preflight는 `imap_mailbox_state` row를 잠궈 IMAP UID
allocator끼리는 직렬화한다. 다만 API/수신/복구 경로가 같은 folder에
새 active message를 삽입하면 capacity 계산과 실제 lazy backfill 사이의
backlog가 달라질 수 있다. 메시지 insert가 참조하는 folder row까지 같은
트랜잭션에서 잠궈, UID capacity 계산과 backfill 대상 집합을 하나의
직렬화된 mailbox mutation 구간으로 만들어야 한다.

### 구현 대상

- `internal/maildb/imap_append.go`
- `internal/maildb/imap_uid.go`
- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] UID capacity preflight가 mailbox state row와 함께 folder row를 `FOR UPDATE`로 잠근다.
- [x] cross-mailbox MOVE destination backfill helper도 folder row lock을 잡고 backlog를 읽는다.
- [x] 기존 APPEND/COPY/MOVE lazy UID ordering/exhaustion 회귀 테스트가 같은 잠금 순서에서 통과한다.
- [x] `go test ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-278: IMAP lazy UID lock ordering audit
