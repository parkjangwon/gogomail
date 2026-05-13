# ACTIVE_TASK

## TASK-295: IMAP event broker diagnostics concurrency audit

### 배경

IMAP mailbox event broker에 aggregate/per-mailbox drop counter와 subscriber
count 진단 메서드가 추가됐다. 이 메서드들은 운영 중 publish, subscribe, cancel과
동시에 호출될 수 있으므로, 브로커 mutex 계약을 통해 상태 읽기와 쓰기가 안전하게
공존해야 한다. 동시 publish/cancel/diagnostic 호출 회귀 테스트로 이 계약을
고정한다.

### 구현 대상

- `internal/imapgw/events.go`
- `internal/imapgw/events_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] publish와 drop counter 진단 메서드가 동시에 호출되는 테스트를 추가한다.
- [x] subscribe/cancel과 subscriber count 진단 메서드가 동시에 호출되는 테스트를 추가한다.
- [x] 동시성 테스트 종료 후 subscriber count가 0으로 수렴하는지 검증한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-296: IMAP event broker diagnostics race audit
