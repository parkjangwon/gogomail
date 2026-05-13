# ACTIVE_TASK

## TASK-244: SMTP inbound domain policy multi-recipient audit

### 배경

SMTP receive 경로는 첫 번째로 해석된 수신자의 도메인 정책만 세션에 캐시해 같은
메일 트랜잭션의 이후 수신자 도메인 정책을 반영하지 못했다. 다중 수신자 메일에서
두 번째 이후 도메인의 더 엄격한 수신자 수/크기 제한이 우회될 수 있으므로,
수신자별 정책을 조회하고 세션 전체에 가장 엄격한 enforce 정책을 집계해야 한다.

### 구현 대상

- `internal/smtp/receiver.go`
- `internal/smtp/policy.go`
- `internal/smtp/status.go`
- `internal/smtp/receiver_test.go`
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/backend-roadmap.md`

### 완료 조건

- [x] RCPT마다 해당 수신자 도메인 정책을 조회한다.
- [x] 다중 수신자 세션에서 가장 엄격한 enforce 정책을 집계한다.
- [x] 도메인 정책 조회 실패 시 fail-open 하지 않고 임시 SMTP 오류를 반환한다.
- [x] `go test ./internal/smtp` 통과.
- [x] `go test ./...` 통과.
- [x] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-245: SMTP object storage orphan rollback audit
