# ACTIVE_TASK

## TASK-289: IMAP event broker per-mailbox drop accounting audit

### 배경

IMAP mailbox event broker는 slow-subscriber 드롭 수를 aggregate로 노출한다.
하지만 동일 서버에 여러 사용자와 mailbox 구독이 동시에 있을 때 aggregate 값만
있으면 어느 selected mailbox에서 드롭이 발생했는지 확인하기 어렵다. 운영 진단을
위해 user/mailbox 단위 드롭 카운터를 함께 유지하고 정규화된 identity로 조회할 수
있어야 한다.

### 구현 대상

- `internal/imapgw/events.go`
- `internal/imapgw/events_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] IMAP event broker가 slow subscriber 드롭을 user/mailbox 단위로 누적한다.
- [x] user/mailbox 단위 드롭 카운터를 trim-aware 조회 메서드로 제공한다.
- [x] non-blocking slow subscriber 테스트가 aggregate와 mailbox별 드롭 카운터를 함께 검증한다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-290: IMAP event broker canceled publish validation audit
