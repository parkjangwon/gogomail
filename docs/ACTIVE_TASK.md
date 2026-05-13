# ACTIVE_TASK

## TASK-296: IMAP event broker diagnostics race audit

### 배경

IMAP mailbox event broker 진단 메서드와 동시성 테스트가 추가됐다. 단위 테스트가
통과해도 Go race detector로 publish, subscribe, cancel, diagnostic 경로의 실제
메모리 접근 경쟁 여부를 별도로 확인해야 한다. IMAP gateway 패키지에 race detector
게이트를 실행해 브로커 동시성 계약을 검증한다.

### 구현 대상

- `internal/imapgw/events.go`
- `internal/imapgw/events_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] `go test -race -count=1 ./internal/imapgw` 통과.
- [x] IMAP event broker 동시성 진단 테스트가 race detector 아래에서 실행된다.
- [x] `go test ./internal/imapgw` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-297: IMAP mailbox event server ignore audit
