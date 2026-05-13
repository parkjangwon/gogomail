# ACTIVE_TASK

## TASK-302: IMAP mailbox event zero-message initial EXISTS audit

### 배경

legacy EXISTS 이벤트는 `Messages=0`을 "정확한 0개"가 아니라 "새 메시지 하나가
도착했다"는 호환 신호로 처리한다. selected mailbox count가 0인 초기 상태에서도
이 경로는 `* 1 EXISTS`와 `selectedMessages=1`로 이어져야 한다. 초기 0개 상태의
legacy increment 동작을 별도 테스트로 고정한다.

### 구현 대상

- `internal/imapgw/server_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] selected count 0에서 `Messages=0` legacy EXISTS 이벤트가 `* 1 EXISTS`를 만드는지 검증한다.
- [x] selected count 0에서 legacy EXISTS 이벤트가 `selectedMessages=1`로 갱신하는지 검증한다.
- [x] 초기 0개 mailbox의 legacy increment 경로가 별도 테스트로 고정된다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-303: IMAP mailbox event expunge zero-sequence audit
