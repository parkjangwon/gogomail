# ACTIVE_TASK

## TASK-276: IMAP lazy UID backfill exhaustion audit

### 배경

lazy UID backfill과 APPEND/COPY/MOVE의 새 UID 배정은 같은 mailbox UID
공간을 소비한다. `uidnext`가 32-bit IMAP UID 한계에 가까울 때 backfill
개수와 새 메시지 개수를 합산하지 않으면, 중간 insert 뒤 state check
constraint나 DB 오류에 기대게 된다. 트랜잭션 안에서 UID state를 잠근 뒤
필요한 UID 수를 미리 계산해 명확한 exhaustion 오류로 실패해야 한다.

### 구현 대상

- `internal/maildb/imap_append.go`
- `internal/maildb/imap_uid.go`
- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] APPEND/COPY/same-mailbox MOVE가 lazy backfill backlog와 신규 UID 수를 합산해 capacity를 사전 검증한다.
- [x] cross-mailbox MOVE destination backfill helper도 UIDNEXT 저장 가능 범위를 기준으로 overflow를 차단한다.
- [x] PostgreSQL 회귀 테스트가 APPEND/COPY overflow에서 `imap uid space exhausted`로 실패하고 UID row를 남기지 않음을 검증한다.
- [x] `go test ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-277: IMAP lazy UID capacity race audit
