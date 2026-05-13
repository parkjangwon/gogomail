# ACTIVE_TASK

## TASK-290: IMAP event broker canceled publish validation audit

### 배경

IMAP mailbox event broker는 publish 시작 시 context cancellation을 확인한다.
slow-subscriber 드롭 카운터가 추가된 이후에도, 취소된 publish는 fanout뿐 아니라
aggregate 및 mailbox별 드롭 카운터에도 어떤 부작용을 남기면 안 된다. 이
불변식을 회귀 테스트로 고정해 context 취소가 브로커 상태 변경보다 앞선다는
계약을 보장한다.

### 구현 대상

- `internal/imapgw/events.go`
- `internal/imapgw/events_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 취소된 publish context가 mailbox 이벤트를 fanout하지 않는 테스트를 추가한다.
- [x] 취소된 publish context가 aggregate 드롭 카운터를 증가시키지 않는 테스트를 추가한다.
- [x] 취소된 publish context가 mailbox별 드롭 카운터를 증가시키지 않는 테스트를 추가한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-291: IMAP event broker canceled subscription validation audit
