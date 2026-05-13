# ACTIVE_TASK

## TASK-308: IMAP mailbox event expunge empty-selected race audit

### 배경

IMAP EXPUNGE empty-selected 방어는 `writeMailboxEvent`, IDLE live, NOOP drain
경로에서 검증됐다. 이벤트 경로는 goroutine과 net.Pipe 기반 테스트가 섞이므로,
Go race detector로 empty-selected EXPUNGE 변경 이후에도 IMAP gateway 패키지의
동시성 안전성을 확인해야 한다.

### 구현 대상

- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `go test -race -count=1 ./internal/imapgw` 통과.
- [x] empty-selected EXPUNGE IDLE/NOOP 통합 테스트가 race detector 아래에서 실행된다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-309: POP3 auth policy freshness audit
