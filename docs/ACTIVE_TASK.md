# ACTIVE_TASK

## TASK-294: IMAP event broker validation side-effect audit

### 배경

IMAP mailbox event broker는 publish/subscribe 입력을 검증한 뒤에만 내부 상태를
변경해야 한다. 이벤트 type, user ID, mailbox ID 검증이 실패하면 기존 구독자,
fanout 채널, aggregate/per-mailbox 드롭 카운터에 어떤 부작용도 남기면 안 된다.
브로커 validation 경로가 상태 변경보다 앞선다는 계약을 회귀 테스트로 고정한다.

### 구현 대상

- `internal/imapgw/events.go`
- `internal/imapgw/events_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] invalid publish가 기존 subscriber에게 이벤트를 fanout하지 않는지 검증한다.
- [x] invalid publish가 aggregate/per-mailbox drop 카운터를 변경하지 않는지 검증한다.
- [x] invalid subscribe가 subscriber count를 증가시키지 않는지 검증한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-295: IMAP event broker diagnostics concurrency audit
