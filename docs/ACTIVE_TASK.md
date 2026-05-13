# ACTIVE_TASK

## TASK-292: IMAP event broker cancel idempotency accounting audit

### 배경

IMAP mailbox event broker의 subscription cancel 함수는 `sync.Once`로 보호된다.
하지만 구독자 수 진단 메서드가 추가된 만큼, cancel을 여러 번 호출해도 channel
close와 subscribers map 제거가 한 번만 일어나고 구독자 수가 0으로 유지되는지
회귀 테스트로 고정해야 한다.

### 구현 대상

- `internal/imapgw/events.go`
- `internal/imapgw/events_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] subscription cancel을 여러 번 호출해도 panic 없이 완료되는 테스트를 추가한다.
- [x] 반복 cancel 후 구독자 수가 0으로 유지되는지 검증한다.
- [x] 반복 cancel 후 subscription channel이 닫힌 상태인지 검증한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-293: IMAP event broker context cancel idempotency audit
