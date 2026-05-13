# ACTIVE_TASK

## TASK-301: IMAP mailbox event legacy EXISTS increment audit

### 배경

최신 IMAP EXISTS 이벤트는 `Messages`에 절대 mailbox count를 담지만, 기존 producer
또는 테스트 backend는 `Messages=0`인 legacy EXISTS 이벤트를 보낼 수 있다. 서버는
이 경우 현재 selected count를 1 증가시키는 호환 경로를 유지한다. 절대 count 경로와
legacy increment 경로가 명확히 구분되도록 회귀 테스트로 고정한다.

### 구현 대상

- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `Messages=0` legacy EXISTS 이벤트가 `selectedMessages+1` wire response를 만드는지 검증한다.
- [x] legacy EXISTS 이벤트가 `selectedMessages`를 1 증가시키는지 검증한다.
- [x] legacy increment 경로가 절대 count 경로와 별도 테스트로 고정된다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-302: IMAP mailbox event zero-message initial EXISTS audit
