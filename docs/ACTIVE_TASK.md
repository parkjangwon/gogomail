# ACTIVE_TASK

## TASK-297: IMAP mailbox event server ignore audit

### 배경

IMAP server는 selected mailbox 이벤트를 NOOP drain과 IDLE live 경로에서 소비한다.
이때 다른 user 또는 다른 mailbox의 이벤트는 반드시 무시해야 하며, selected
message count나 wire response에 영향을 주면 안 된다. 브로커가 identity를
정규화하더라도 서버 측 필터가 정확히 동작한다는 것을 회귀 테스트로 고정한다.

### 구현 대상

- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] NOOP drain 경로가 다른 user/mailbox 이벤트를 wire response에 출력하지 않는지 검증한다.
- [x] IDLE live 경로가 다른 user/mailbox 이벤트를 wire response에 출력하지 않는지 검증한다.
- [x] selected mailbox 이벤트만 EXISTS response로 반영되는지 검증한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-298: IMAP mailbox event unknown-type server audit
