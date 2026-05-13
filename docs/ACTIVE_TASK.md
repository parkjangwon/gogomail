# ACTIVE_TASK

## TASK-426: SMTP inbound domain policy QUIT DSN isolation audit

### 배경

SMTP inbound 수신에서 `QUIT`으로 닫힌 연결의 envelope, DSN 옵션, recipient-domain 정책
누적 상태는 다른 TCP 연결로 새면 안 된다. 이전 연결의 DSN metadata가 다음 연결의 성공
트랜잭션으로 새면 안 된다.

### 구현 대상

- `internal/smtp/protocol_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 실제 SMTP 프로토콜 연결에서 DSN 옵션을 포함한 d1+d2 혼합 도메인 트랜잭션 후 `QUIT`하는 첫 연결을 구성한다.
- [x] 새 TCP 연결의 d1-only `DATA`가 이전 d2 size 제한에 막히지 않고 성공 기록되는지 검증한다.
- [x] 기록된 성공 메시지에 이전 연결의 DSN envelope/recipient 옵션이 남지 않는지 검증한다.
- [x] `go test -count=1 ./internal/smtp` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-427: SMTP inbound domain policy auth reset audit
