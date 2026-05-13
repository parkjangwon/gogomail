# ACTIVE_TASK

## TASK-268: IMAP lazy UID assignment ordering audit

### 배경

IMAP lazy UID assignment는 active message에 UID를 뒤늦게 배정하더라도
클라이언트가 보는 sequence number와 UID ordering이 일관되어야 한다. 특히
`AfterUID` 기반 부분 목록을 반환할 때 sequence number를 부분 목록 index로
암묵 대체하면 실제 mailbox 기준 sequence와 어긋난다.

### 구현 대상

- `internal/maildb/imap_uid.go`
- `internal/maildb/imap_uid_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `ListIMAPMessages`가 UID 정렬 후 실제 mailbox 기준 sequence number를 명시적으로 채운다.
- [x] `AfterUID` 부분 목록은 이전 active UID 개수를 base로 sequence number를 계산한다.
- [x] `go test ./internal/maildb` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-269: POP3 message listing consistency audit
