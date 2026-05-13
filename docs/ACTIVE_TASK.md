# ACTIVE_TASK

## TASK-279: IMAP ensure-message UID lock ordering audit

### 배경

`EnsureIMAPMessageUID`는 LIST/FETCH 등에서 단일 메시지에 lazy UID를
부여하는 핵심 경로다. APPEND/COPY/MOVE/backfill 경로는 state → folder →
message 잠금 순서와 UID capacity preflight를 갖췄지만, 단일 메시지
경로가 이를 따르지 않으면 near-limit에서 DB constraint 오류에 기대거나
운영 backfill과 다른 잠금 순서를 만들 수 있다.

### 구현 대상

- `internal/maildb/imap_append.go`
- `internal/maildb/imap_uid.go`
- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `EnsureIMAPMessageUID`가 UID state row 다음 folder row를 잠근 뒤 target message를 검사한다.
- [x] 실제 UID assignment CTE가 target message row를 `FOR UPDATE OF m`으로 잠근다.
- [x] 단일 메시지 lazy UID 배정이 existing UID는 그대로 반환하면서, 신규 UID가 필요한 경우 UIDNEXT capacity를 사전 검증한다.
- [x] PostgreSQL 회귀 테스트가 UID exhaustion 및 message-row lock 대기를 검증하고 UID row를 남기지 않음을 확인한다.
- [x] `go test ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-280: IMAP batch ensure UID ordering audit
