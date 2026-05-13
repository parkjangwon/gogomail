# ACTIVE_TASK

## TASK-305: IMAP mailbox event expunge empty-selected audit

### 배경

IMAP EXPUNGE 이벤트는 selected mailbox에 메시지가 있을 때만 유효하다. 기존 서버
경로는 `selectedMessages=0`인 상태에서 `SequenceNumber>0` 이벤트가 들어오면
decrement는 하지 않지만 `* 1 EXPUNGE` 같은 유효하지 않은 response를 보낼 수
있었다. 비어 있는 selected mailbox에서는 EXPUNGE 이벤트를 조용히 무시해야 한다.

### 구현 대상

- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `selectedMessages=0`이면 EXPUNGE 이벤트가 wire response를 만들지 않는다.
- [x] `selectedMessages=0`이면 EXPUNGE 이벤트가 selected count를 변경하지 않는다.
- [x] out-of-range clamp 경로는 selected count가 0보다 클 때만 적용된다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-306: IMAP mailbox event expunge empty-selected IDLE audit
