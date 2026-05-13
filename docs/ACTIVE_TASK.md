# ACTIVE_TASK

## TASK-414: SMTP inbound domain policy size lookup failure audit

### 배경

SMTP inbound 수신에서 혼합 도메인 RCPT 중 하나의 도메인 정책 조회가 실패하면 해당 RCPT는
fail-closed로 거절되어야 한다. 다만 이미 수락된 이전 수신자와 누적된 정상 정책 상태가
오염되어서는 안 되며, 이후 `DATA`는 수락된 수신자만 대상으로 처리되어야 한다.

### 구현 대상

- `internal/smtp/receiver_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 첫 번째 수신자 도메인 정책 조회는 성공하고 두 번째 수신자 도메인 정책 조회는 실패하는 세션을 구성한다.
- [x] 두 번째 `RCPT`가 `451 4.7.1`로 fail-closed 되는지 검증한다.
- [x] 이후 `DATA`가 첫 번째 수락 수신자만 대상으로 성공 기록되는지 검증한다.
- [x] `go test -count=1 ./internal/smtp` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-415: SMTP inbound mixed-domain policy reset audit
