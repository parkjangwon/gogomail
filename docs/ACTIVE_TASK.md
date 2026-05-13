# ACTIVE_TASK

## TASK-421: SMTP inbound domain policy DATA failure DSN reset audit

### 배경

SMTP inbound 수신에서 혼합 도메인 정책 때문에 `DATA`가 실패하면 envelope, DSN 옵션,
recipient-domain 정책 누적 상태가 모두 초기화되어야 한다. 실패한 트랜잭션의 DSN metadata가
다음 성공 트랜잭션으로 새면 안 된다.

### 구현 대상

- `internal/smtp/receiver_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] DSN 옵션을 포함한 d1+d2 혼합 도메인 트랜잭션이 d2 size 제한으로 `552 5.3.4`를 반환하는지 검증한다.
- [x] d1-only `DATA`가 이전 d2 size 제한에 막히지 않고 성공 기록되는지 검증한다.
- [x] 기록된 성공 메시지에 실패한 트랜잭션의 DSN envelope/recipient 옵션이 남지 않는지 검증한다.
- [x] `go test -count=1 ./internal/smtp` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-422: SMTP inbound domain policy RSET DSN reset audit
