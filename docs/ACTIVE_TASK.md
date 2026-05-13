# ACTIVE_TASK

## TASK-273: IMAP move lazy UID destination ordering audit

### 배경

IMAP MOVE도 목적지 mailbox의 UID namespace에서 새 UID를 발급하고,
소스 UID는 expunge 된다. 목적지에 active 상태지만 `imap_message_uid`가
없는 메시지가 있으면 STATUS 예측은 그 메시지를 포함하지만 MOVE가
목적지 미할당 UID를 먼저 backfill 하지 않으면 COPYUID-style MOVE 응답과
후속 LIST/FETCH 순서가 예측보다 낮은 UID를 볼 수 있다.

### 구현 대상

- `internal/maildb/imap_uid.go`
- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] MOVE 목적지 트랜잭션이 기존 active 미할당 메시지 UID를 먼저 backfill 한다.
- [x] 이동된 메시지는 backfill된 목적지 기존 메시지 다음 UID와 sequence number를 받는다.
- [x] PostgreSQL 회귀 테스트가 목적지 STATUS 예측, MOVE 목적지 UID 결과, LIST 순서, 최종 UIDNEXT/HIGHESTMODSEQ를 검증한다.
- [x] `go test ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-274: IMAP same-mailbox move lazy UID ordering audit
