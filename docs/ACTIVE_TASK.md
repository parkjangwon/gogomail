# ACTIVE_TASK

## TASK-413: SMTP inbound mixed-domain policy audit

### 배경

SMTP inbound 수신에서 한 메시지가 여러 도메인의 수신자를 포함하면, 수신자 도메인들의
enforce 정책을 누적해서 가장 엄격한 제한을 적용해야 한다. recipient count 제한은 이미
고정되어 있으므로, 이번 감사는 두 번째 수신자 도메인의 더 작은 message size 제한이
`DATA` 단계에도 적용되는지 확인한다.

### 구현 대상

- `internal/smtp/receiver_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] 첫 번째 수신자 도메인보다 두 번째 수신자 도메인의 message size 제한이 더 작은 혼합 도메인 세션을 구성한다.
- [x] 두 수신자 모두 `RCPT` 단계에서 수락되는지 검증한다.
- [x] `DATA` 단계에서 누적된 더 엄격한 도메인 message size 제한으로 `552 5.3.4`가 반환되는지 검증한다.
- [x] `go test -count=1 ./internal/smtp` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-414: SMTP inbound domain policy size lookup failure audit
