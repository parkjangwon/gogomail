# ACTIVE_TASK

## TASK-300: IMAP mailbox event fresh EXISTS audit

### 배경

IMAP EXISTS 이벤트가 현재 selected count보다 큰 `Messages` 값을 포함하면 서버는
그 값을 정확한 mailbox count로 채택하고 클라이언트에 `* N EXISTS`를 보내야 한다.
복구/수신 이벤트가 정확한 count를 포함하도록 보강된 만큼, 서버가 fresh count를
증분이 아니라 절대 count로 반영하는지 직접 테스트로 고정한다.

### 구현 대상

- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 현재 selected count보다 큰 EXISTS count가 `* N EXISTS` wire response를 만드는지 검증한다.
- [x] fresh EXISTS 이벤트가 `selectedMessages`를 event `Messages` 값으로 갱신하는지 검증한다.
- [x] fresh EXISTS count가 단순 +1이 아니라 절대 count로 반영되는지 검증한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-301: IMAP mailbox event legacy EXISTS increment audit
