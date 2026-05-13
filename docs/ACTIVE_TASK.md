# ACTIVE_TASK

## TASK-252: IMAP COPY lifecycle integration audit

### 배경

IMAP `COPY`는 복사할 메시지 UID 집합이 비어 있어도 destination mailbox 인자를
검증해야 한다. 현재 `$` SEARCHRES 등으로 UID가 0개가 된 경로는 대상 mailbox
존재 여부를 확인하기 전에 `OK COPY completed`를 반환할 수 있다.

### 구현 대상

- `internal/imapgw/server.go`
- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 빈 UID 집합의 COPY도 destination mailbox를 먼저 조회한다.
- [x] destination이 없으면 빈 UID 집합이어도 `[TRYCREATE]` 응답을 반환한다.
- [x] destination이 존재하고 UID 집합이 비어 있는 경우에는 기존처럼 OK를 유지한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-253: POP3 auth password-change policy audit
