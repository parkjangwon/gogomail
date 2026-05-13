# ACTIVE_TASK

## TASK-306: IMAP mailbox event expunge empty-selected IDLE audit

### 배경

IMAP EXPUNGE empty-selected 방어는 `writeMailboxEvent` 단위에서 보강됐다. 같은
방어가 실제 IDLE live 이벤트 경로에서도 wire response를 만들지 않는지 확인해야
한다. 빈 mailbox를 SELECT한 세션이 IDLE 중 EXPUNGE 이벤트를 받아도 클라이언트는
EXPUNGE를 받지 않고 DONE 완료 응답만 받아야 한다.

### 구현 대상

- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 빈 mailbox SELECT 후 IDLE 중 EXPUNGE 이벤트가 wire response를 만들지 않는지 검증한다.
- [x] empty-selected EXPUNGE 이후 DONE이 정상 완료되는지 검증한다.
- [x] IDLE 통합 테스트용 빈 mailbox event backend를 추가한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-307: IMAP mailbox event expunge empty-selected NOOP audit
