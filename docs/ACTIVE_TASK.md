# ACTIVE_TASK

## TASK-303: IMAP mailbox event expunge zero-sequence audit

### 배경

IMAP EXPUNGE 이벤트는 sequence number가 없으면 클라이언트에 유효한 EXPUNGE
response를 만들 수 없다. 서버는 `SequenceNumber=0` 이벤트를 조용히 무시해야 하며,
selected message count나 saved SEARCH sequence state를 변경하면 안 된다. 이
방어 경로를 직접 테스트로 고정한다.

### 구현 대상

- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `SequenceNumber=0` EXPUNGE 이벤트가 wire response를 만들지 않는지 검증한다.
- [x] `SequenceNumber=0` EXPUNGE 이벤트가 `selectedMessages`를 감소시키지 않는지 검증한다.
- [x] `SequenceNumber=0` EXPUNGE 이벤트가 saved SEARCH sequence state를 변경하지 않는지 검증한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-304: IMAP mailbox event expunge clamp audit
