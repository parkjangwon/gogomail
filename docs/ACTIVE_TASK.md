# ACTIVE_TASK

## TASK-271: IMAP append lazy UID ordering audit

### 배경

IMAP mailbox 상태 조회는 기존 active 메시지 중 아직 `imap_message_uid`
행이 없는 메시지를 `UIDNEXT`/`HIGHESTMODSEQ` 예측에 포함한다. 하지만
APPEND 저장 트랜잭션이 기존 미할당 메시지를 먼저 UID backfill 하지 않으면
새 APPEND 메시지가 예측보다 낮은 UID를 받을 수 있다. APPENDUID,
STATUS UIDNEXT, 이후 FETCH/LIST 순서가 같은 mailbox UID timeline을
보도록 저장 경계를 정리해야 한다.

### 구현 대상

- `internal/maildb/imap_append.go`
- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] APPEND 트랜잭션이 기존 active 미할당 메시지 UID를 먼저 backfill 한다.
- [x] 새 APPEND 메시지는 backfill된 기존 메시지 다음 UID와 sequence number를 받는다.
- [x] PostgreSQL 회귀 테스트가 STATUS 예측, APPENDUID, LIST 순서, 최종 UIDNEXT/HIGHESTMODSEQ를 검증한다.
- [x] `go test ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-272: IMAP copy lazy UID destination ordering audit
