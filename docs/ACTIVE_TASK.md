# ACTIVE_TASK

## TASK-298: IMAP mailbox event unknown-type server audit

### 배경

IMAP mailbox event broker는 unknown event type을 거부하지만, 테스트 backend나
다른 구현체가 server subscription channel에 직접 unknown type을 보낼 수 있다.
서버의 `writeMailboxEvent`는 unknown type을 무시하도록 되어 있으므로, NOOP drain과
IDLE live 경로에서 selected mailbox unknown 이벤트가 wire response나 selected
message count에 영향을 주지 않는지 회귀 테스트로 고정한다.

### 구현 대상

- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] NOOP drain 경로가 selected mailbox unknown event type을 wire response에 출력하지 않는지 검증한다.
- [x] IDLE live 경로가 selected mailbox unknown event type을 wire response에 출력하지 않는지 검증한다.
- [x] unknown event 뒤의 정상 EXISTS 이벤트만 selected message count로 반영되는지 검증한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-299: IMAP mailbox event stale EXISTS audit
