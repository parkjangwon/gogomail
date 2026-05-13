# ACTIVE_TASK

## TASK-304: IMAP mailbox event expunge clamp audit

### 배경

IMAP EXPUNGE 이벤트의 sequence number가 현재 selected message count보다 크면
클라이언트에 존재하지 않는 sequence를 보낼 수 없다. 서버는 이 값을 현재 selected
count로 clamp한 뒤 EXPUNGE response를 보내고, saved SEARCH state도 clamp된 sequence
기준으로 갱신해야 한다. 이 방어 경로를 직접 테스트로 고정한다.

### 구현 대상

- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] selected count보다 큰 EXPUNGE sequence가 selected count로 clamp된 wire response를 만드는지 검증한다.
- [x] clamped EXPUNGE 이벤트가 `selectedMessages`를 1 감소시키는지 검증한다.
- [x] saved SEARCH state가 clamp된 sequence 기준으로 갱신되는지 검증한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-305: IMAP mailbox event expunge empty-selected audit
