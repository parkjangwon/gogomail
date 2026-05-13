# ACTIVE_TASK

## TASK-417: SMTP inbound domain policy MAIL reset audit

### 배경

SMTP inbound 수신에서 새 `MAIL` 명령은 이전 트랜잭션의 envelope와 혼합 도메인 정책 누적
상태를 초기화해야 한다. d1+d2 수신자를 추가한 뒤 새 `MAIL`로 전환하면, 다음 d1-only
트랜잭션은 이전 d2 size 제한 없이 정상 처리되어야 한다.

### 구현 대상

- `internal/smtp/receiver_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] d1+d2 혼합 도메인 RCPT를 추가한 뒤 새 `MAIL`을 수행하는 세션을 구성한다.
- [x] 같은 세션에서 새 `MAIL` 이후 d1-only `RCPT` 트랜잭션을 시작할 수 있는지 검증한다.
- [x] d1-only `DATA`가 이전 d2 size 제한에 막히지 않고 성공 기록되는지 검증한다.
- [x] `go test -count=1 ./internal/smtp` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-418: SMTP inbound domain policy EHLO reset audit
