# ACTIVE_TASK

## TASK-288: IMAP event broker slow-subscriber metrics audit

### 배경

IMAP mailbox event broker는 느린 구독자가 publisher를 막지 않도록 가득 찬
구독자 채널에는 non-blocking으로 이벤트를 드롭한다. 이 동작은 서버 안정성에는
필요하지만, 드롭 수를 관측할 방법이 없어 운영 중 selected session 이벤트 유실
압력을 확인하기 어렵다. 브로커 내부에 드롭 카운터를 두어 테스트와 운영 진단에서
slow-subscriber 압력을 확인할 수 있게 해야 한다.

### 구현 대상

- `internal/imapgw/events.go`
- `internal/imapgw/events_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] IMAP event broker가 slow subscriber로 드롭한 이벤트 수를 누적한다.
- [x] 드롭 카운터를 읽을 수 있는 안전한 접근자를 제공한다.
- [x] non-blocking slow subscriber 테스트가 드롭 카운터를 검증한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-289: IMAP event broker per-subscriber drop accounting audit
