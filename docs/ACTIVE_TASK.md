# ACTIVE_TASK

## TASK-420: SMTP inbound domain policy QUIT isolation audit

### 배경

SMTP inbound 수신에서 `QUIT`으로 닫힌 연결의 envelope와 혼합 도메인 정책 누적 상태는
다른 연결로 절대 새면 안 된다. 실제 TCP 프로토콜 경로에서 d1+d2 수신자를 추가한 뒤
`QUIT`하고 새 연결을 만들면, 다음 d1-only 트랜잭션은 이전 d2 size 제한 없이 정상 처리되어야
한다.

### 구현 대상

- `internal/smtp/protocol_integration_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 실제 SMTP 프로토콜 연결에서 d1+d2 혼합 도메인 RCPT를 추가한 뒤 `QUIT`하는 첫 연결을 구성한다.
- [x] 새 TCP 연결에서 `MAIL`/d1-only `RCPT` 트랜잭션을 시작할 수 있는지 검증한다.
- [x] d1-only `DATA`가 이전 d2 size 제한에 막히지 않고 성공 기록되는지 검증한다.
- [x] `go test -count=1 ./internal/smtp` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-421: SMTP inbound domain policy DATA failure DSN reset audit
