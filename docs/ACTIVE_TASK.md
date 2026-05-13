# ACTIVE_TASK

## TASK-293: IMAP event broker context cancel idempotency audit

### 배경

IMAP mailbox event broker는 구독 context가 취소되면 내부 goroutine이
subscription cancel 함수를 호출한다. 이 경로 역시 explicit cancel과 같은
`sync.Once`를 공유하므로, context cancel이 여러 번 발생하고 이후 explicit cancel을
호출해도 구독자 accounting이 0으로 유지되어야 한다. context cancellation 경로의
idempotency를 테스트로 고정한다.

### 구현 대상

- `internal/imapgw/events.go`
- `internal/imapgw/events_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] context cancel을 여러 번 호출해도 subscription channel이 닫히는지 검증한다.
- [x] context cancel 후 구독자 수가 0으로 유지되는지 검증한다.
- [x] context cancel 이후 explicit cancel을 호출해도 구독자 수가 0으로 유지되는지 검증한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-294: IMAP event broker validation side-effect audit
