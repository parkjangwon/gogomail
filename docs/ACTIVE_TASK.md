# ACTIVE_TASK

## TASK-274: IMAP same-mailbox move lazy UID ordering audit

### 배경

같은 mailbox로 MOVE 하는 경우에도 구현은 새 메시지/새 UID를 만들고
기존 UID를 삭제해 RFC 6851의 COPYUID/EXPUNGE 흐름을 유지한다. 이때 같은
mailbox 안에 active 상태지만 `imap_message_uid`가 없는 메시지가 있으면
새 UID 배정 전에 먼저 backfill 해야 STATUS 예측, MOVE destination UID,
EXPUNGE 후 sequence number가 같은 UID timeline에 남는다.

### 구현 대상

- `internal/maildb/imap_uid.go`
- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] same-mailbox MOVE 트랜잭션이 같은 mailbox의 기존 active 미할당 메시지 UID를 먼저 backfill 한다.
- [x] 새로 만들어진 moved 메시지는 backfill된 기존 메시지 다음 UID를 받고 EXPUNGE 후 올바른 sequence number를 받는다.
- [x] PostgreSQL 회귀 테스트가 STATUS 예측, MOVE destination UID, 원본 UID 제거, LIST 순서, 최종 UIDNEXT/HIGHESTMODSEQ를 검증한다.
- [x] `go test ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-275: IMAP lazy UID backfill helper consolidation audit
