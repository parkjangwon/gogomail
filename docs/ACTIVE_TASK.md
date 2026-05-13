# ACTIVE_TASK

## TASK-280: IMAP batch ensure UID ordering audit

### 배경

`EnsureIMAPMessageUIDsForMessages`는 restore/exists 이벤트를 위해 여러
메시지의 lazy UID를 보장한다. 내부적으로 단일-message ensure를 반복하므로
요청 순서가 mailbox의 시간순서와 다르면 최신 메시지가 더 낮은 UID를 받을
수 있다. batch 대상은 요청 순서가 아니라 mailbox별 internal date/id 순서로
정렬해, 운영 backfill 및 LIST lazy assignment와 같은 UID timeline을 따라야
한다.

### 구현 대상

- `internal/maildb/imap_append.go`
- `internal/maildb/imap_uid.go`
- `internal/maildb/postgres_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] batch ensure 대상 조회가 mailbox, internal date, message ID 순으로 정렬된다.
- [x] 역순 요청에서도 같은 mailbox의 오래된 메시지가 낮은 UID를 받는다.
- [x] PostgreSQL 회귀 테스트가 reversed request order에서도 UID 1/2가 mailbox order로 배정됨을 검증한다.
- [x] `go test ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-281: IMAP restored EXISTS event ordering audit
