# ACTIVE_TASK

## TASK-278: IMAP lazy UID lock ordering audit

### 배경

IMAP UID 배정 경로가 state row, folder row, message row를 서로 다른
순서로 잡으면 운영 backfill과 실시간 APPEND/COPY/MOVE 사이에서 불필요한
잠금 대기나 deadlock 위험이 생길 수 있다. capacity preflight에서 도입한
state → folder → message 잠금 순서를 수동/운영 UID backfill 경로에도
맞춰 mailbox UID mutation의 잠금 순서를 통일한다.

### 구현 대상

- `internal/maildb/imap_append.go`
- `internal/maildb/imap_uid.go`
- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 운영용 `BackfillIMAPMailboxUIDs`가 state row 다음 folder row를 잠근 뒤 message rows를 선택한다.
- [x] lazy UID allocation/backfill 경로의 주요 잠금 순서를 state → folder → message로 맞춘다.
- [x] 기존 IMAP UID backfill 및 APPEND/COPY/MOVE 회귀 테스트가 같은 잠금 순서에서 통과한다.
- [x] `go test ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-279: IMAP ensure-message UID lock ordering audit
