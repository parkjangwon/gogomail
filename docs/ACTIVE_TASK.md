# ACTIVE_TASK

## TASK-286: IMAP event broker identity normalization audit

### 배경

IMAP mailbox event broker는 구독과 발행 모두에서 user/mailbox ID가 비어
있는지 trim 검증하지만, 실제 저장 및 비교에는 trim 전 원본 값을 사용한다.
따라서 `" user-1 "` 또는 `" inbox "`처럼 공백이 섞인 입력은 검증을 통과하면서
구독자와 발행 이벤트가 서로 매칭되지 않을 수 있다. 브로커 입구에서 identity를
정규화해 모든 producer/consumer가 같은 비교 규칙을 공유해야 한다.

### 구현 대상

- `internal/imapgw/events.go`
- `internal/imapgw/events_test.go`
- `internal/mailservice/service_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] IMAP event broker 구독 user/mailbox ID가 trim된 값으로 저장된다.
- [x] IMAP event broker 발행 user/mailbox ID가 trim된 값으로 fanout된다.
- [x] 공백이 섞인 구독/발행 identity가 정상 매칭되는 테스트를 추가한다.
- [x] mailservice 구독 wrapper 테스트가 새 정규화 계약을 검증한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./internal/mailservice` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-287: IMAP event broker event type normalization audit
