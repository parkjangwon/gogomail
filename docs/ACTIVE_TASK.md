# ACTIVE_TASK

## TASK-299: IMAP mailbox event stale EXISTS audit

### 배경

IMAP EXISTS 이벤트는 selected mailbox의 메시지 수를 증가시키는 방향으로만
클라이언트에 알려야 한다. 서버는 `Messages` 값이 현재 selected count 이하이면
stale 이벤트로 보고 무시한다. 이 동작이 wire response와 internal selected count를
바꾸지 않는지 직접 테스트로 고정한다.

### 구현 대상

- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 현재 selected count보다 작은 EXISTS count가 wire response를 만들지 않는지 검증한다.
- [x] 현재 selected count와 같은 EXISTS count가 wire response를 만들지 않는지 검증한다.
- [x] stale EXISTS 이벤트가 `selectedMessages`를 변경하지 않는지 검증한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-300: IMAP mailbox event fresh EXISTS audit
