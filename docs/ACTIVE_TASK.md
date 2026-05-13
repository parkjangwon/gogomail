# ACTIVE_TASK

## TASK-291: IMAP event broker canceled subscription validation audit

### 배경

IMAP mailbox event broker는 subscribe 시작 시 context cancellation을 확인한다.
이미 취소된 context로 Subscribe가 호출되면 channel/cancel 함수가 생성되지 않고
브로커 내부 subscribers map에도 흔적이 남지 않아야 한다. 이 불변식을 테스트할 수
있도록 구독자 수를 안전하게 조회하는 진단 메서드도 함께 제공한다.

### 구현 대상

- `internal/imapgw/events.go`
- `internal/imapgw/events_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 취소된 subscribe context가 channel/cancel 함수를 만들지 않는 테스트를 추가한다.
- [x] 브로커 구독자 수를 안전하게 조회하는 진단 메서드를 제공한다.
- [x] 취소된 subscribe context가 구독자 수를 증가시키지 않는 테스트를 추가한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-292: IMAP event broker cancel idempotency accounting audit
