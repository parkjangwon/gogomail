# ACTIVE_TASK

## TASK-251: IMAP SELECT snapshot consistency audit

### 배경

IMAP 세션이 이미 selected 상태일 때 다른 `SELECT`/`EXAMINE`이 실패하면 기존
메일함 선택 상태가 유지되어야 한다. 현재 구현은 새 mailbox 조회 전에 먼저
deselect를 수행해 실패한 재선택이 기존 selected 상태와 subscription을 잃게 만든다.

### 구현 대상

- `internal/imapgw/server.go`
- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 실패한 `SELECT`/`EXAMINE`이 기존 selected mailbox 상태를 지우지 않는다.
- [x] 성공한 재선택은 새 mailbox 응답/구독 준비 뒤 기존 subscription을 교체한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-252: IMAP COPY lifecycle integration audit
