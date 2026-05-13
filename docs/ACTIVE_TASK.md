# ACTIVE_TASK

## TASK-275: IMAP lazy UID no-op mutation audit

### 배경

COPY와 same-mailbox MOVE의 lazy UID backfill은 실제 source 메시지가 있을
때만 실행되어야 한다. 요청 UID가 모두 없는 no-op 명령인데 목적지 또는
같은 mailbox의 기존 미할당 메시지를 backfill 하면, 클라이언트가 요청한
변경은 없는데 UIDNEXT/HIGHESTMODSEQ 저장 상태만 바뀌는 부작용이 생긴다.
UID timeline 정합성 보강은 유지하되 no-op 명령은 저장소 mutation을 만들지
않도록 CTE 의존성을 명확히 해야 한다.

### 구현 대상

- `internal/maildb/imap_uid.go`
- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] COPY destination lazy backfill은 실제 복사 source가 있을 때만 실행된다.
- [x] same-mailbox MOVE lazy backfill은 실제 이동 source가 있을 때만 실행된다.
- [x] PostgreSQL 회귀 테스트가 no-op COPY/MOVE에서 UID row와 mailbox stored state가 바뀌지 않음을 검증한다.
- [x] `go test ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-276: IMAP lazy UID backfill exhaustion audit
